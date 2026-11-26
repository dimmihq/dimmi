::embed{src="javascript:alert(1)"}

::embed{src="data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg=="}

::embed{src="https://evil.example/embed/dQw4w9WgXcQ"}

::embed{src="https://www.youtube.com.evil.example/watch?v=dQw4w9WgXcQ"}

::embed{src="https://user@www.youtube.com/watch?v=dQw4w9WgXcQ"}

::embed{src="https://www.youtube.com/watch?v=dQw4w9Wg%22%3E%3Cscript%3E"}

::embed{src="https://a.b.wistia.com/medias/abcdefghij"}

::embed[<script>alert(1)</script>]{src="https://youtu.be/dQw4w9WgXcQ"}

::embed["><img src=x onerror=alert(1)>]{src="https://youtu.be/dQw4w9WgXcQ"}

::embed{src="https://youtu.be/dQw4w9WgXcQ" onload="alert(1)" sandbox="allow-top-navigation"}

::button[click]{href="javascript:alert(1)"}

::button[click]{href="JaVaScRiPt:alert(1)"}

::button[click]{href="//evil.example/x"}

::button[click]{href="data:text/html,<script>alert(1)</script>"}

::button[<img src=x onerror=alert(1)>]{href="https://example.com"}

::button[x]{href="https://example.com\" onclick=\"alert(1)"}

:::details[<script>alert(1)</script>]
<img src=x onerror=alert(1)>
:::

:::details["><svg onload=alert(1)>]
[x](javascript:alert(1))
:::

![x](javascript:alert(1) "caption")

![x](/a.png "<script>alert(1)</script>")

<iframe src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"></iframe>

<details open ontoggle=alert(1)><summary>x</summary></details>
