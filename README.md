# kslb

> Kubernetes Service Loadbalance. 按比例均衡负载服务流量。

### Why kslb?

Q: 在 Kubernetes 中如何实现按比例负载服务流量？

A: 在原生的 Kubernetes 中，要实现这种行为只能通过 deployments 实例数均衡负载。比如 v1/v2/v3 版本要想按照 3:2:1 的比例均衡请求，就需要 v1/v2/v3 部署的实例数为 3:2:1，如果我只想 3 个版本都只部署 1 个实例的话，那就没办法了。目前成熟的方案，ServiceMesh! 不过上手 ServiceMesh 有一定的成本并且整一套东西的太大太重了。所以应该有一种更轻便的实现方式。

### What's kslb?

kslb 是利用 Nginx 实现的基于 L4 做负载转发的服务组件，可以通过在 svc -> endpoint 中间在新增一层代理来实现上述需求。

#### 优点

* 轻便，性能强，Nginx 本身是一个无状态应用，支持水平扩展。
* 对后端无感知，后端版本或是权重变动不会影响前端入口。
* Nginx 配置热更。

#### 缺点

* 可定制性差，没有其他负载功能，缺少像 ServiceMesh 那样完善的熔断/限流/根据具体规则（如 Header 等其他信息转发）。

#### Kubernetes 原生方案

```
                    |--- instance-v1
                    |--- instance-v1
                    |--- instance-v1
request --> svc --> |--- instance-v2
                    |--- instance-v2
                    |--- instance-v3
```

#### kslb 方案

```
                              |--- svc-v1(weight1) --> instance-v1
request --> svc --> nginx --> |--- svc-v2(weight2) --> instance-v2
                              |--- svc-v3(weight3) --> instance-v3
```

### How kslb?

为了测试用途，先部署几个不同版本的 web 服务。
```shell
# 项目位于 https://github.com/chenjiandongx/example-app
~ 🐶 k apply -f example/app.yaml
~ 🐶 k get pods | grep appv
appv1-5757db6d6c-smc5m                    1/1     Running   0          167m
appv2-586d975694-2dbp9                    1/1     Running   0          167m
appv3-78c4bff6c7-4mjbk                    1/1     Running   0          167m

~ 🐶 k get svc | grep appv
appv1-svc                   ClusterIP   10.96.126.172    <none>        8080/TCP         167m
appv2-svc                   ClusterIP   10.104.31.253    <none>        8080/TCP         167m
appv3-svc                   ClusterIP   10.102.93.135    <none>        8080/TCP         167m
```

部署 kslb
```yaml
# example/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kslb
spec:
  selector:
    matchLabels:
      name: kslb
  template:
    metadata:
      labels:
        name: kslb
    spec:
      containers:
        - name: kslb
          image: chenjiandongx/kslb:latest
          imagePullPolicy: IfNotPresent
          volumeMounts:
            - name: myapp-nginx-config
              mountPath: /etc/kslb
      # 需要挂载配置文件，配置文件变动 kslb 会启动 nginx reload
      volumes:
        - name: myapp-nginx-config
          configMap:
            name: myapp-nginx-config
---
# configMap 是定义转发规则的配置文件
apiVersion: v1
kind: ConfigMap
metadata:
  name: myapp-nginx-config
data:
  svc.yaml: |
    # ports: array int
    # 声明需要监听转发的端口
    #
    # servers: array obj{host: $host, weight: $weight}
    # 声明后端服务 svc 以及权重
    ports:
    - 8080
    servers:
    - host: appv1-svc.default
      weight: 3
    - host: appv2-svc.default
      weight: 2
    - host: appv3-svc.default
      weight: 1
---
apiVersion: v1
kind: Service
metadata:
  name: app-svc
spec:
  ports:
    - port: 8080
      protocol: TCP
      targetPort: 8080
  selector:
    name: kslb
  type: ClusterIP

# kubectl apply -f example/deployment.yaml
```

测试结果，由于 kslb 实例部署的 svc 为 ClusterIP，所以需要在集群容器内访问（也可以改为 NodePort/LB 类型）
```shell
bash-4.2$ for i in {1..100}; do curl -s http://app-svc.default:8080; done > /tmp/out.log; cat /tmp/out.log | grep v1 | echo v1-count `wc -l`; cat /tmp/out.log | grep v2 | echo v2-count `wc -l`; cat /tmp/out.log | grep v3 | echo v3-count `wc -l`;
v1-count 50
v2-count 33
v3-count 17
bash-4.2$ for i in {1..100}; do curl -s http://app-svc.default:8080; done > /tmp/out.log; cat /tmp/out.log | grep v1 | echo v1-count `wc -l`; cat /tmp/out.log | grep v2 | echo v2-count `wc -l`; cat /tmp/out.log | grep v3 | echo v3-count `wc -l`;
v1-count 50
v2-count 34
v3-count 16
bash-4.2$ for i in {1..100}; do curl -s http://app-svc.default:8080; done > /tmp/out.log; cat /tmp/out.log | grep v1 | echo v1-count `wc -l`; cat /tmp/out.log | grep v2 | echo v2-count `wc -l`; cat /tmp/out.log | grep v3 | echo v3-count `wc -l`;
v1-count 50
v2-count 33
v3-count 17
bash-4.2$ for i in {1..100}; do curl -s http://app-svc.default:8080; done > /tmp/out.log; cat /tmp/out.log | grep v1 | echo v1-count `wc -l`; cat /tmp/out.log | grep v2 | echo v2-count `wc -l`; cat /tmp/out.log | grep v3 | echo v3-count `wc -l`;
v1-count 50
v2-count 33
v3-count 17
```

修改比例为 1:1:1
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: myapp-nginx-config
data:
  svc.yaml: |
    ports:
    - 8080
    servers:
    - host: appv1-svc.default
      weight: 1
    - host: appv2-svc.default
      weight: 1
    - host: appv3-svc.default
      weight: 1
```

大概会有 10s 左右的延迟，再次校验结果
```shell
bash-4.2$ for i in {1..100}; do curl -s http://app-svc.default:8080; done > /tmp/out.log; cat /tmp/out.log | grep v1 | echo v1-count `wc -l`; cat /tmp/out.log | grep v2 | echo v2-count `wc -l`; cat /tmp/out.log | grep v3 | echo v3-count `wc -l`;
v1-count 34
v2-count 33
v3-count 33
bash-4.2$ for i in {1..100}; do curl -s http://app-svc.default:8080; done > /tmp/out.log; cat /tmp/out.log | grep v1 | echo v1-count `wc -l`; cat /tmp/out.log | grep v2 | echo v2-count `wc -l`; cat /tmp/out.log | grep v3 | echo v3-count `wc -l`;
v1-count 33
v2-count 34
v3-count 33
bash-4.2$ for i in {1..100}; do curl -s http://app-svc.default:8080; done > /tmp/out.log; cat /tmp/out.log | grep v1 | echo v1-count `wc -l`; cat /tmp/out.log | grep v2 | echo v2-count `wc -l`; cat /tmp/out.log | grep v3 | echo v3-count `wc -l`;
v1-count 33
v2-count 33
v3-count 34
bash-4.2$ for i in {1..100}; do curl -s http://app-svc.default:8080; done > /tmp/out.log; cat /tmp/out.log | grep v1 | echo v1-count `wc -l`; cat /tmp/out.log | grep v2 | echo v2-count `wc -l`; cat /tmp/out.log | grep v3 | echo v3-count `wc -l`;
v1-count 34
v2-count 33
v3-count 33
```

### License
MIT [©chenjiandongx](https://github.com/chenjiandongx)

