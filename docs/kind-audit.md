# kind con audit log

El guardia necesita saber QUIEN cambio que. En kind el audit log del
API server viene desactivado; este repo levanta el cluster con audit
habilitado desde el principio mediante `scripts/kind/create-cluster.sh`
y la policy `scripts/kind/audit-policy.yaml`.

## Levantar (o recrear) el cluster

```bash
bash scripts/kind/create-cluster.sh
```

El script es idempotente: borra el cluster `umbra` si existe y lo vuelve
a crear (imagen `kindest/node:v1.37.0`) con estos flags extra del API server:

- `audit-log-path: /var/log/umbra-audit/audit.log`
- `audit-policy-file: /etc/kubernetes/umbra-audit-policy.yaml`
  (montado desde `scripts/kind/audit-policy.yaml` del repo)
- rotacion: `audit-log-maxage=7`, `audit-log-maxbackup=3`, `audit-log-maxsize=100`

Al terminar deja el contexto `kind-umbra` seleccionado y espera a que el
nodo este `Ready`.

## Donde vive el log y como leerlo

El log vive DENTRO del nodo control-plane, en la ruta fija
`/var/log/umbra-audit/audit.log`. Se lee con `docker exec`
(`kubectl exec` no sirve: apunta a pods, y el control-plane es el
contenedor del nodo, no un pod):

```bash
docker exec umbra-control-plane cat /var/log/umbra-audit/audit.log
```

Filtrar por un objeto concreto (ej. un patch sobre un deployment de demo):

```bash
docker exec umbra-control-plane cat /var/log/umbra-audit/audit.log \
  | grep '"namespace":"demo"'
```

## Ejemplo esperado

```bash
kubectl create namespace demo
kubectl create deployment demo-audit --image=ghcr.io/example/demo:1 --namespace=demo
kubectl patch deployment demo-audit --namespace=demo \
  -p '{"spec":{"replicas":2}}'
docker exec umbra-control-plane cat /var/log/umbra-audit/audit.log \
  | grep '"name":"demo-audit"'
```

Una linea del log (JSON, una entrada por linea) contiene:

```json
{
  "user": {"username": "kubernetes-admin"},
  "verb": "patch",
  "objectRef": {
    "resource": "deployments",
    "namespace": "demo",
    "name": "demo-audit"
  }
}
```

Es decir: `user` (quien), `verb` (que operacion) y `objectRef`
(sobre que objeto). Limpieza del ejemplo:

```bash
kubectl delete namespace demo
```

## Que registra la policy (y que no)

`scripts/kind/audit-policy.yaml`, nivel `Metadata`, `omitStages:
[RequestReceived]`:

- Registra: verbos `create`, `update` y `patch` sobre workloads
  (`deployments`, `deployments/scale`, `statefulsets`, `daemonsets`,
  `replicasets`, `pods`) en el namespace `demo`.
- Descarta (`level: None`): todo lo demas, incluidas TODAS las lecturas
  (`get`/`list`/`watch`). Si hay que auditar mas namespaces de demo,
  se anaden a la lista `namespaces` de la regla.

## Sin secretos

Ningun archivo de esta tarea contiene credenciales, endpoints privados
ni material sensible: solo la imagen publica de ejemplo
(`ghcr.io/example/*`), el contexto `kind-umbra` y rutas dentro del nodo.
