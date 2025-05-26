## 运维工具箱

目前支持的功能有
* 获取集群中各节点的资源使用情况 
```
kubectl-ops getNodeResource
```
* 获取集群中各pod资源占用情况
```shell
kubectl-ops getPodResource [-n namespace | --node node_name | --sort CPURequest,desc]

#kubectl-ops getPodResource
Usage:
  ops getPodResource [flags]

Flags:
  -h, --help               help for getPodResource
  -n, --namespace string   get pod resource in specific namespace
      --node string        get pod resource in specific node
  -s, --sort string        sort pod resource using key desc, key,value (e.g. CPURequest,desc)
                           supported Keys: Name, Namespace, NodeName,CPURequest, CPULimit, CPUUsage, MemRequest, MemLimit, MemUsage
                           supported values: desc, asc
  -t, --target string      get pod resource of specific workload (default "sts")

Global Flags:
  -k, --kubeconfig string   Kubeconfig 文件路径 (default "/root/.kube/config")
```

* 分析pod调度失败原因
```shell
kubectl-ops why <Pod_Name> [-n namespace]

# kubectl ops why --help 
show why pod cannot be scheduled

Usage:
  ops why podname -n namespace [flags]

Flags:
  -h, --help               help for why
  -n, --namespace string   get pod resource in specific namespace (default "default")

Global Flags:
  -k, --kubeconfig string   Kubeconfig 文件路径 (default "/root/.kube/config")
```

## quick start
```shell
##生成二进制可部署到linux服务器
make build 

## 方便本地调试
make local-build 
```