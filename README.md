![kirk-rage](https://github.com/mitjafelicijan/jbmafp/assets/296714/b0e745ec-97dd-474d-836b-ee3c34759015)

# Just Build Me A Fucking Page

I am just so sick of all these complicated static site generators forcing you to
care about taxonomies and shit like this. All I want is to have a bunch of
markdown files and let them use specific templates. That is about it. Nothing
fancy!

This generator is not for people who need something more complicated. Use Hugo
instead. But if you need a simple blog page that needs to spit out an RSS feed
or two and have the option to define different templates for different posts,
well then this might be useful to you.

The only thing hard about this project is the spelling of its name.

Some facts (will be more clear when you read the whole readme):

- You cannot nest your markdown file under `content` folder. All files must be
  in the root of `content` folder.
- `public` folder gets automatically created on `jbmafp --build`.
- All files in `static` folder will be moved to the root of `public` folder.
- When you provide `url` in your markdown files, this will create these files in
  the root of `public` folder. No nesting allowed.
- Comes with a small embedded HTTP server you can invoke with `jbmafo --server`
  which will server contents from `public` folder. Good for testing stuff.

## Install

```sh
git clone git@github.com:mitjafelicijan/jbmafp.git
cd jbmafp
go install .
```

## Generate first site

- Go to your projects folder or wherever you want to place the site.

```sh
mkdir my-ugly-website
cd my-ugly-website
jbmafp --init
jbmafp --build
```

- Check out `public` folder and you will see a website. That is about it.
- You can also do `jbmafp --help` to see all the option.

## Understanding all this bullshit

- Posts go into `content` folder.
- Each post must have fields defined between `---` block. All of the fields are
  required. If you have ever used Hugo, this is the same thing. Below is example
  `content/first.md`.

```md
---
title: "My first post"
url: first.html
date: 2023-06-29T14:51:39+02:00
type: post
draft: false
---

This is my first post. It ain't much but it's an honest post.
```

- `type` is used all over the place. It is used to define a template file of the
  page that will be generated. If type is `post` then the program will load
  `templates/post.html` to handle generation of the page.
- You can use whatever name you want. I use `note`, `post` as types to separate
  all the pages into categories.
- `type` is also used inside templates like:
  ```html
  <ul>
	{{ range .Pages }}
	{{ if eq .Type "post" }}
	<li><a href="/{{ .RelPermalink }}">{{ .Title }}</a></li>
	{{ end }}
	{{ end }}
  </ul>
  ```
- This is also use for generating RSS feed. Check `templates/index.xml` to see
  the example.
- This opens door to quite versatile build option.
- You can trigger additional generation of content under `extras` field in
  `config.yaml` file. RSS feed gets generated this way. `template` field tells
  generator which file in `templates` folder to use and `url` tells generator
  what the file should be called when its saved.

## License

[jbmafp](https://github.com/mitjafelicijan/jbmafp) was written by [Mitja
Felicijan](https://mitjafelicijan.com) and is released under the BSD two-clause
license, see the LICENSE file for more information.
