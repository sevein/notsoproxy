# notsoproxy

Proxy server written in Go after the talk given by Mark Smith (@zorkian) at
linux.conf.au (2015).

The server implements a simple pool of backend connections with a queue based
on Go channels.

Readers and writers are buffered for performance reasons, to avoid frequent IO.

There is a global map that provide a simple statistics mechanism. Access to the
map is synchronized with a mutex to avoid unexpected behaviors, e.g. crashes. Go
maps are not controlled, so this was a necessary step because the map is
accessed from different goroutines. It would be possible to obtain similar
results with Go channels (share memory by communicating).
