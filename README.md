# Space

> A likely-to-be-unfinished experimental IndieWeb site, built with Go.

Space is an attempt at building an IndieWeb-capable personal site, before I
started looking into AT Protocol. It uses [Chi](https://go-chi.io/#/),
[Templ](https://templ.guide/), and [TailwindCSS](https://tailwindcss.com).

Media files are stored in an S3 backend of your choice, and any unrecognized
or unsupported [post types](https://indieweb.org/posts) are be rendered as raw
JSON until support was added.

Notably, the Tailwind pipeline is not checked into the project, as this was
built before Go Tool support.

If you want to take it for a test-drive, you can check the `.env.example` file
for all required configuration values. At the time, I was quite pleased with
how this turned out.

## Supported Specs

- [x] [IndieAuth](https://www.w3.org/TR/indieauth/) via Basic Auth
- [x] [Micropub](https://www.w3.org/TR/micropub/)
- [ ] [h-card](https://microformats.org/wiki/h-card)
- [ ] [Webmentions](https://www.w3.org/TR/webmention/)
- [ ] [Microsub](https://indieweb.org/Microsub-spec)
