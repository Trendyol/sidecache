# sidecache
Sidecar cache for kubernetes applications. It acts as a proxy sidecar between application and client, routes incoming requests to cache storage or application according to Istio VirtualService routing rules.

[![License: MIT](https://img.shields.io/badge/License-MIT-ligthgreen.svg)](https://opensource.org/licenses/MIT)

### Istio Configuration for Routing Http Requests to Sidecar Container

Below VirtualService is responsible for routing all get requests to port 9191 on your pod, other http requests goes to port 8080.

```
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: foo
spec:
  gateways:
  - foo-gateway
  hosts:
  - foo
  http:
  - match:
    - method:
        exact: GET
    route:
    - destination:
        host: foo
        port:
          number: 9191
  - route:
    - destination:
        host: foo
        port:
          number: 8080
```
