// lb presents simple client-based LoadBalancer
// provide:
// - distribute request to nodes in node-list
// - circuitBreaker break request when successive error or high concurrent number
// - retry in one server or other server
package slb
