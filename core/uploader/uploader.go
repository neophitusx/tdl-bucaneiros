package uploader

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gabriel-vasile/mimetype"
	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/styling"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/iyear/tdl/core/util/fsutil"
	"github.com/iyear/tdl/core/util/mediautil"
)

// MaxPartSize refer to https://core.telegram.org/api/files#uploading-files
const MaxPartSize = 512 * 1024

type Uploader struct {
	opts Options
}

type Options struct {
	Client   *tg.Client
	Threads  int
	Iter     Iter
	Progress Progress
}

func New(o Options) *Uploader {
	return &Uploader{opts: o}
}

func (u *Uploader) Upload(ctx context.Context, limit int) error {
	wg, wgctx := errgroup.WithContext(ctx)
	wg.SetLimit(limit)

	for u.opts.Iter.Next(wgctx) {
		elem := u.opts.Iter.Value()

		wg.Go(func() (rerr error) {
			u.opts.Progress.OnAdd(elem)
			defer func() { u.opts.Progress.OnDone(elem, rerr) }()

			if err := u.upload(wgctx, elem); err != nil {
				// canceled by user, so we directly return error to stop all
				if errors.Is(err, context.Canceled) {
					return errors.Wrap(err, "upload")
				}

				// don't return error, just log it
			}

			return nil
		})
	}

	if err := u.opts.Iter.Err(); err != nil {
		return errors.Wrap(err, "iter")
	}

	return wg.Wait()
}

func (u *Uploader) upload(ctx context.Context, elem Elem) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	up := uploader.NewUploader(u.opts.Client).
		WithPartSize(MaxPartSize).
		WithThreads(u.opts.Threads).
		WithProgress(&wrapProcess{
			elem:    elem,
			process: u.opts.Progress,
		})

	f, err := up.Upload(ctx, uploader.NewUpload(elem.File().Name(), elem.File(), elem.File().Size()))
	if err != nil {
		return errors.Wrap(err, "upload file")
	}

	if _, err = elem.File().Seek(0, io.SeekStart); err != nil {
		return errors.Wrap(err, "seek file")
	}
	mime, err := mimetype.DetectReader(elem.File())
	if err != nil {
		return errors.Wrap(err, "detect mime")
	}

	// here convert underlying entities to formatters for message caption
	caption := styling.Custom(func(eb *entity.Builder) error {
		msg, entities := elem.Caption()
		eb.Format(msg, lo.Map(entities, func(item tg.MessageEntityClass, _ int) entity.Formatter {
			return func(_, _ int) tg.MessageEntityClass {
				return item
			}
		})...)
		return nil
	})

	var thumbFile tg.InputFileClass
	if thumb, ok := elem.Thumb(); ok {
		thumbFile, err = uploadThumbnail(ctx, u.opts.Client, thumb)
		if err != nil {
			return errors.Wrap(err, "upload thumbnail")
		}
	}

	var media message.MediaOption

	if elem.Spoiler() {
		// gotd's UploadedDocument/UploadedPhoto builders don't expose the
		// Spoiler field, so build the raw tg structs to send the media hidden
		// behind a spoiler warning.
		media, err = spoilerMedia(elem, mime, f, thumbFile, caption)
		if err != nil {
			return err
		}
	} else {
		doc := message.UploadedDocument(f, caption).MIME(mime.String()).Filename(elem.File().Name())
		if thumbFile != nil {
			doc = doc.Thumb(thumbFile)
		}

		media = doc

		switch {
		case elem.AsVideo():
			// Force video streaming upload (e.g. for MKV files).
			// Try to get metadata via FFmpeg; if unavailable just upload with basic Video attributes.
			vDoc := doc.Video().SupportsStreaming()
			if vinfo, err := mediautil.GetVideoInfoFFmpeg(elem.FilePath()); err == nil {
				vDoc = vDoc.
					Duration(time.Duration(vinfo.Duration)*time.Second).
					Resolution(vinfo.Width, vinfo.Height)
			} else {
				// Non-fatal: warn and proceed without metadata
				_, _ = fmt.Fprintf(os.Stderr, "warning: could not extract video metadata: %v\n", err)
			}
			media = vDoc
		case mediautil.IsImage(mime.String()) && elem.AsPhoto():
			// webp should be uploaded as document
			if mime.String() == "image/webp" {
				break
			}
			// upload as photo
			media = message.UploadedPhoto(f, caption)
		case mediautil.IsVideo(mime.String()):
			// reset reader
			if _, err = elem.File().Seek(0, io.SeekStart); err != nil {
				return errors.Wrap(err, "seek file")
			}
			if dur, w, h, err := mediautil.GetMP4Info(elem.File()); err == nil {
				// #132. There may be some errors, but we can still upload the file
				media = doc.Video().
					Duration(time.Duration(dur)*time.Second).
					Resolution(w, h).
					SupportsStreaming()
			}
		case mediautil.IsAudio(mime.String()):
			media = doc.Audio().Title(fsutil.GetNameWithoutExt(elem.File().Name()))
		}
	}

	_, err = message.NewSender(u.opts.Client).
		WithUploader(up).
		To(elem.To()).
		Reply(elem.Thread()).
		Media(ctx, media)
	if err != nil {
		return errors.Wrap(err, "send message")
	}

	return nil
}

// spoilerMedia builds the raw tg media structs with the Spoiler field set, since
// gotd's UploadedDocument/UploadedPhoto builders don't expose it.
func spoilerMedia(elem Elem, mime *mimetype.MIME, f tg.InputFileClass, thumb tg.InputFileClass, caption styling.StyledTextOption) (message.MediaOption, error) {
	// photos are uploaded as inputMediaUploadedPhoto
	if mime.String() != "image/webp" && mediautil.IsImage(mime.String()) && elem.AsPhoto() {
		photo := &tg.InputMediaUploadedPhoto{
			Spoiler: true,
			File:    f,
		}
		photo.SetFlags()

		return message.Media(photo, caption), nil
	}

	attrs := []tg.DocumentAttributeClass{
		&tg.DocumentAttributeFilename{FileName: elem.File().Name()},
	}

	switch {
	case elem.AsVideo():
		vAttr := &tg.DocumentAttributeVideo{SupportsStreaming: true}
		if vinfo, err := mediautil.GetVideoInfoFFmpeg(elem.FilePath()); err == nil {
			vAttr.Duration = (time.Duration(vinfo.Duration) * time.Second).Seconds()
			vAttr.W = vinfo.Width
			vAttr.H = vinfo.Height
		} else {
			// Non-fatal: warn and proceed without metadata
			_, _ = fmt.Fprintf(os.Stderr, "warning: could not extract video metadata: %v\n", err)
		}
		attrs = append(attrs, vAttr)
	case mediautil.IsVideo(mime.String()):
		if _, err := elem.File().Seek(0, io.SeekStart); err != nil {
			return nil, errors.Wrap(err, "seek file")
		}
		if dur, w, h, err := mediautil.GetMP4Info(elem.File()); err == nil {
			// #132. There may be some errors, but we can still upload the file
			attrs = append(attrs, &tg.DocumentAttributeVideo{
				Duration:          (time.Duration(dur) * time.Second).Seconds(),
				W:                 w,
				H:                 h,
				SupportsStreaming: true,
			})
		}
	case mediautil.IsAudio(mime.String()):
		attrs = append(attrs, &tg.DocumentAttributeAudio{
			Title: fsutil.GetNameWithoutExt(elem.File().Name()),
		})
	}

	document := &tg.InputMediaUploadedDocument{
		Spoiler:    true,
		File:       f,
		Thumb:      thumb,
		MimeType:   mime.String(),
		Attributes: attrs,
	}
	document.SetFlags()

	return message.Media(document, caption), nil
}

const (
	thumbnailMaxDimension = 320
	thumbnailMaxSize      = 200 * 1024
)

// uploadThumbnail converts a JPEG or PNG thumbnail to Telegram's required JPEG
// format, bounds its dimensions and uploads it as an InputFile.
func uploadThumbnail(ctx context.Context, client *tg.Client, thumb File) (tg.InputFileClass, error) {
	if _, err := thumb.Seek(0, io.SeekStart); err != nil {
		return nil, errors.Wrap(err, "seek thumbnail")
	}

	img, _, err := image.Decode(thumb)
	if err != nil {
		return nil, errors.Wrap(err, "decode thumbnail")
	}

	img = resizeThumbnail(img)
	data, err := encodeThumbnail(img)
	if err != nil {
		return nil, err
	}

	return uploader.NewUploader(client).
		WithPartSize(MaxPartSize).
		FromBytes(ctx, "thumb.jpg", data)
}

func resizeThumbnail(src image.Image) image.Image {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= thumbnailMaxDimension && height <= thumbnailMaxDimension {
		return src
	}

	newWidth, newHeight := thumbnailMaxDimension, thumbnailMaxDimension
	if width > height {
		newHeight = height * thumbnailMaxDimension / width
	} else {
		newWidth = width * thumbnailMaxDimension / height
	}

	src = imaging.Resize(src, newWidth, newHeight, imaging.Lanczos)

	return imaging.Sharpen(src, 0.8)
}

func encodeThumbnail(img image.Image) ([]byte, error) {
	for _, quality := range []int{95, 85, 75, 65, 55, 45, 35, 25} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, errors.Wrap(err, "encode thumbnail as JPEG")
		}
		if buf.Len() <= thumbnailMaxSize {
			return buf.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("thumbnail exceeds Telegram's %d KB limit after JPEG compression", thumbnailMaxSize/1024)
}
