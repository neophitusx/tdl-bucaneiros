---
title: "Upload"
weight: 40
---

# Upload

## Upload Files

Upload specified files and directories to `Saved Messages`:

{{< command >}}
tdl up -p /path/to/file -p /path/to/dir
{{< /command >}}

## Custom Destination

Upload to custom chat.

{{< include "snippets/chat.md" >}}

### Specific Chat

Upload to specific one chat:

{{< command >}}
tdl up -p /path/to/file -c CHAT
{{< /command >}}

Upload to specific topic in a forum chat:

{{< command >}}
tdl up -p /path/to/file -c CHAT --topic TOPIC_ID
{{< /command >}}

### Message Routing

Upload to different chats by message router which is based on [expression](/reference/expr).

{{< hint warning >}}
The `--to` flag is conflicted with the `-c/--chat` and `--topic` flags. You can only use one of them.
{{< /hint >}}

List all available fields:

{{< command >}}
tdl up -p /path/to/file --to -
{{< /command >}}

Upload to `CHAT1` if MIME contains `video`, otherwise upload to `Saved Messages`:

{{< hint info >}}
You must return a **string** or **struct** as the target CHAT, and empty string means upload to `Saved Messages`.
{{< /hint >}}

{{< command >}}
tdl up -p /path/to/file \
--to 'MIME contains "video" ? "CHAT1" : ""'
{{< /command >}}

Upload to `CHAT1` if MIME contains `video`, otherwise upload to reply to message/topic `4` in `CHAT2`:

{{< command >}}
tdl up -p /path/to/file \
--to 'MIME contains "video" ? "CHAT1" : { Peer: "CHAT2", Thread: 4 }'
{{< /command >}}

Pass a file name if the expression is complex:

{{< details "router.txt" >}}
Write your expression like `switch`:

```javascript
MIME contains "video" ? "CHAT1" :
FileExt contains ".mp3" ? "CHAT2" :
FileName contains "chat3" > 30 ? {Peer: "CHAT3", Thread: 101} :
""
```

{{< /details >}}

{{< command >}}
tdl up -p /path/to/file --to router.txt
{{< /command >}}

## Custom Parameters

Upload with 8 threads per task, 4 concurrent tasks:

{{< command >}}
tdl up -p /path/to/file -t 8 -l 4
{{< /command >}}

## Custom Caption

Custom caption is based on [expression](/reference/expr).

By default, the caption is the uploaded filename (including its extension) in monospace formatting. Override it with `--caption` when needed.

## Upload Video with a Thumbnail

Use `--as-video` to send a file such as MKV as a streaming video. `ffprobe` must be available in `PATH` so tdl can provide the duration and dimensions to Telegram. Pass `--thumb` with a JPEG or PNG path to use a custom video thumbnail; PNG files are converted to Telegram-compatible JPEG thumbnails automatically. Use `--thumb auto` to extract a frame at 10% of the video duration (between 10 and 60 seconds); this also requires `ffmpeg` in `PATH`.

```bash
tdl upload -p /path/to/video.mkv --as-video --thumb /path/to/thumbnail.png

# Generate a thumbnail automatically
tdl upload -p /path/to/video.mkv --as-video --thumb auto
```

List all available fields:

{{< command >}}
tdl up -p /path/to/file --caption -
{{< /command >}}

Custom simple caption:
{{< command >}}
tdl up -p /path/to/file --caption 'FileName + " - uploaded by tdl"'
{{< /command >}}

Write styled message with [HTML](https://core.telegram.org/bots/api#html-style):
{{< command >}}
tdl up -p /path/to/file --caption  \
'FileName + `<b>Bold</b> <a href="https://example.com">Link</a>`'
{{< /command >}}

Pass a file name if the expression is complex:

{{< details "caption.txt" >}}

```javascript
repeat(FileName, 2) + `
<a href="https://www.google.com">Google</a>
<a href="https://www.bing.com">Bing</a>
<b>bold</b>
<i>italic</i>
<code>code</code>
<tg-spoiler>spoiler</tg-spoiler>
<pre><code class="language-go">
package main

import "fmt"

func main() {
    fmt.Println("hello world")
}
</code></pre>
` + MIME
```

{{< /details >}}

{{< command >}}
tdl up -p /path/to/file --caption caption.txt
{{< /command >}}

## Filters

Upload files with extension filters:

{{< hint warning >}}
The extension is only matched with the file name, not the MIME type. So it may not work as expected.

Whitelist and blacklist can not be used at the same time.
{{< /hint >}}

Whitelist: Only upload files with `.jpg` `.png` extension

{{< command >}}
tdl up -p /path/to/file -p /path/to/dir -i jpg,png
{{< /command >}}

Blacklist: Upload all files except `.mp4` `.flv` extension

{{< command >}}
tdl up -p /path/to/file -p /path/to/dir -e mp4 -e flv
{{< /command >}}

## Delete Local

Delete the uploaded file after uploading successfully:

{{< command >}}
tdl up -p /path/to/file --rm
{{< /command >}}

## Photo

Upload images as photos instead of documents:

{{< command >}}
tdl up -p /path/to/file --photo
{{< /command >}}
