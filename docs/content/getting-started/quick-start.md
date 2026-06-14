---
title: "Quick start"
description: "Fetch your first record with openparliament."
weight: 30
---

Once `openparliament` is on your `PATH`, fetch a page. The argument is the path
of the page on openparliament.com (everything after the host), or a full URL:

```bash
openparliament page <path>
```

By default you get an aligned table. Ask for JSON when you want to pipe it:

```bash
$ openparliament page <path> -o json
[
  {
    "id": "<path>",
    "url": "https://openparliament.com/<path>",
    "title": "<path>",
    "body": "..."
  }
]
```

## Shape the output

The same flags work on every command:

```bash
openparliament page <path> --fields id,url        # keep only these columns
openparliament page <path> --template '{{.Body}}' # just the body text
openparliament page <path> -o jsonl | jq .url     # one object per line, into jq
```

`-o` takes `table`, `markdown`, `list`, `json`, `jsonl`, `csv`, `tsv`, `url`, or
`raw`. Left to `auto`, it prints a table to a terminal and JSONL into a pipe, so
the same command reads well by hand and parses cleanly downstream. See
[output formats](/reference/output/) for the full contract.

## Follow the links

`links` lists the pages a page links to, and each one is a path you can fetch in
turn:

```bash
openparliament links <path> -n 10                 # the first ten links
openparliament links <path> -o url                # just the URLs
openparliament links <path> -o url | head -3 | xargs -n1 openparliament page
```

## Search for a page

`search` takes a free-text query and returns matching pages as stubs, each a path
you can fetch in turn:

```bash
openparliament search <query>                     # matching pages
openparliament search <query> -n 5 -o url         # the first five, as URLs
```

## Serve it instead

The same operations are available over HTTP and to agents over MCP:

```bash
openparliament serve --addr :7777 &
curl -s 'localhost:7777/v1/page/<path>'          # NDJSON, one record per line
openparliament mcp                                # MCP over stdio: page, links, search
```

## What to build next

This scaffold ships one example type, `page`, wired end to end so the whole
chain works today. To make it really about openparliament, model the records you
care about in `openparliament/` and declare their operations in
`openparliament/domain.go`. Each one you add shows up as a command here, a route
under `serve`, and a tool under `mcp`, with no extra wiring. The
[guides](/guides/) cover the common jobs.
