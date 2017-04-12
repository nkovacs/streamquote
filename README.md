## streamquote

This package provides a streaming version of `strconv.Quote`.

It allows you to quote the data in an `io.Reader` and write it out to
an `io.Writer` without having to store the entire input
and the entire output in memory.

Its primary use case is go.rice and similar tools, which need to
convert large files to go strings.

```go
converter := streamquote.New()
converter.Convert(inputfile, outfile)
```

Unline `strconv.Quote`, it does not add quotes around the output.
