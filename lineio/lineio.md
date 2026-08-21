# Lineio

Lineio owns one product-neutral mechanism: advancing through an `io.Reader` as
a stream of bounded lines using Go's `bufio.Scanner` and `bufio.ScanLines`.

It validates the initial and maximum line extents before allocation, enforces
one fixed compiler-owned 16 MiB ceiling for both initial allocation and line
growth, preserves native reader and `bufio.ErrTooLong` identities, and leaves
reader ownership and cleanup with the caller. It owns no pool, retry, goroutine,
filesystem access, decoder, or product line grammar.
