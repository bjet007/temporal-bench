load('ext://helm_remote', 'helm_remote')
load("ext://local_output", "local_output")

namespace_name ='bench'

docker_build('ghcr.io/bjet007/temporal-bench', 'worker/', build_args={})

k8s_yaml(helm('helm-chart',
    name='temporal-bench',
    namespace=namespace_name,
    set=[
       'image.repository=ghcr.io/bjet007/temporal-bench',
       'workers=bench\\,basic\\,session-cpu',
       'tests.numDecisionPollers=20',
       'tests.frontendAddress=temporal-frontend.temporal:7233',
       'tests.prometheusURL=http://prometheus-kube-prometheus-prometheus.monitoring:9090'
    ],
))



