# kind with audit log

The guard needs to know WHO changed what. In kind the API server audit log
ships disabled; this repo brings the cluster up with audit enabled from the
start via `scripts/kind/create-cluster.sh`
and the policy `scripts/kind/audit-policy.yaml`.

## Bring up (or recreate) the cluster

```bash
bash scripts/kind/create-cluster.sh
```

The script is idempotent: it deletes the `umbra` cluster if it exists and
recreates it (image `kindest/node:v1.37.0`) with these extra API server flags:

- `audit-log-path: /var/log/umbra-audit/audit.log`
- `audit-policy-file: /etc/kubernetes/umbra-audit-policy.yaml`
  (mounted from the repo's `scripts/kind/audit-policy.yaml`)
- rotation: `audit-log-maxage=7`, `audit-log-maxbackup=3`, `audit-log-maxsize=100`

On finish it leaves the `kind-umbra` context selected and waits until the
node is `Ready`.

## Where the log lives and how to read it

The log lives INSIDE the control-plane node, at the fixed path
`/var/log/umbra-audit/audit.log`. Read it with `docker exec`
(`kubectl exec` doesn't work: it targets pods, and the control-plane is the
node container, not a pod):

```bash
docker exec umbra-control-plane cat /var/log/umbra-audit/audit.log
```

Filter by a concrete object (e.g. a patch on a demo deployment):

```bash
docker exec umbra-control-plane cat /var/log/umbra-audit/audit.log \
  | grep '"namespace":"demo"'
```

## Expected example

```bash
kubectl create namespace demo
kubectl create deployment demo-audit --image=ghcr.io/example/demo:1 --namespace=demo
kubectl patch deployment demo-audit --namespace=demo \
  -p '{"spec":{"replicas":2}}'
docker exec umbra-control-plane cat /var/log/umbra-audit/audit.log \
  | grep '"name":"demo-audit"'
```

One log line (JSON, one entry per line) holds:

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

That is: `user` (who), `verb` (what op) and `objectRef`
(over which object). Clean up the example:

```bash
kubectl delete namespace demo
```

## What the policy records (and what not)

`scripts/kind/audit-policy.yaml`, level `Metadata`, `omitStages:
[RequestReceived]`:

- Records: `create`, `update` and `patch` verbs on workloads
  (`deployments`, `deployments/scale`, `statefulsets`, `daemonsets`,
  `replicasets`, `pods`) in the `demo` namespace.
- Drops (`level: None`): everything else, including ALL reads
  (`get`/`list`/`watch`). To audit more demo namespaces,
  add them to the rule's `namespaces` list.

## No secrets

No file in this task holds credentials, private endpoints
or sensitive material: only the public example image
(`ghcr.io/example/*`), the `kind-umbra` context and paths inside the node.

## Read the log from a pod (read-only, control-plane only)

A reader inside the cluster (e.g. Umbra's future watcher) mounts
the node log read-only and pins to control-plane, the
only node where the file exists:

```yaml
spec:
  nodeSelector:
    kubernetes.io/hostname: umbra-control-plane  # kind: single node
  containers:
    - name: audit-reader
      volumeMounts:
        - name: umbra-audit
          mountPath: /var/log/umbra-audit
          readOnly: true
  volumes:
    - name: umbra-audit
      hostPath:
        path: /var/log/umbra-audit
        type: Directory
```

Rules: `readOnly: true` always (the reader never writes the log) and
`nodeSelector` to control-plane (redundant in single-node kind,
avoids mounting a missing directory in multinode). In a real
cluster no hostPath: audit goes to a webhook backend
(`audit-webhook-*`) or a log agent ships it; the JSON line
format parsed by `internal/attribution` is the same in both cases.
