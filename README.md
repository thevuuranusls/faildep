
# faildep
[![GoDoc](https://godoc.org/github.com/faildep/faildep?status.svg)](https://godoc.org/github.com/faildep/faildep)


    import "github.com/faildep/faildep"

faildep implements common dependence resource failure handling as a basic library.

and it's very easy to implement Resiliency pattern on special resource, like: mysql, redis...

this library provide:

- dispatch request to available resource in resource list
- circuitBreaker break request when successive error or high concurrent number
- retry in one resource or try to do it in other resources


API documentation and examples are available via [godoc](https://godoc.org/github.com/faildep/faildep).
