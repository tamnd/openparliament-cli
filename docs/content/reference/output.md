---
title: "Output formats"
description: "The output contract every command shares: formats, fields, and templates."
weight: 30
---

Every list command in the fleet renders through one formatter, so the same flags
work everywhere. Wire your commands through it as you add them, and this page
describes what users get. Pick a format with `-o`, or let openparliament choose:
a table when writing to a terminal, JSONL when piped.

## Formats

```bash
openparliament <command> -o table     # a rounded, color-aware grid for reading
openparliament <command> -o markdown  # a GitHub pipe table to paste into docs (alias: md)
openparliament <command> -o list      # one record per section, easy on the eyes
openparliament <command> -o jsonl     # one JSON object per line, for piping
openparliament <command> -o json      # a single JSON array
openparliament <command> -o csv       # spreadsheet friendly
openparliament <command> -o tsv       # tab-separated
openparliament <command> -o url       # just the URL column
openparliament <command> -o raw       # the underlying bytes, unformatted
```

| Format | Best for |
|---|---|
| `table` | Reading on a terminal: a rounded border with an accented header |
| `markdown` | Pasting into a README, issue, or PR (alias `md`) |
| `list` | Reading one record at a time: a heading and a short bullet list per record |
| `jsonl` | Piping into another tool, one object at a time |
| `json` | Loading a whole result as an array |
| `csv` / `tsv` | Spreadsheets and quick column math |
| `url` | Feeding URLs into other commands |
| `raw` | The unformatted bytes (response bodies, file contents) |

## Color

On an interactive terminal the `table`, `list`, and `json`/`jsonl` formats are
colored: the table draws a dim border with an accented header, `list` styles each
record's heading and keys, and JSON keys, strings, numbers, and literals are
highlighted. Color is suppressed the moment output is not a terminal, so a pipe
always gets plain, parseable bytes (and `list` falls back to literal Markdown).
Force the choice with `--color always|never` (or set `NO_COLOR`). `markdown`,
`csv`, `tsv`, `url`, and `raw` are never colored, so they stay safe to redirect
into a file.

## Narrowing columns

Keep only the fields you want:

```bash
openparliament <command> --fields id,title,url
```

`--no-header` drops the header row in `table` and `csv` output, which helps when
a downstream tool expects bare rows.

## Templating rows

For full control over each line, apply a Go text/template. Fields are the JSON
keys, capitalised:

```bash
openparliament <command> --template '{{.URL}} {{.Title}}'
```

## Why auto-detection helps

Because the default adapts to the destination, the same command reads well by
hand and parses cleanly in a pipe:

```bash
openparliament <command>            # a table, because this is a terminal
openparliament <command> | wc -l    # JSONL, because this is a pipe
```

You only reach for `-o` when you want something other than that default.
