/**
 * Mock data for the visual alpha of Umbra UI.
 *
 * POLICY (SECURITY.md §4.d): every value here is a dummy placeholder
 * (ghcr.io/example, namespace `demo`, obviously-fake tokens-of-nothing).
 * When the real operator data source lands, findings keep passing through
 * the evidence sanitizer before anything reaches this layer.
 */

export const MOCK_FINDINGS = [
  {
    id: 'F01',
    severity: 'high',
    state: 'verified',
    kind: 'Deployment',
    target: 'demo/web-frontend',
    liveYaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-frontend
  namespace: demo
  labels:
    app.kubernetes.io/name: web-frontend
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: web-frontend
  template:
    spec:
      containers:
        - name: web
          image: ghcr.io/example/web:1.4.2
          ports:
            - containerPort: 8080
              name: http`,
    diffRemoved: [],
    diffAdded: [
      '          resources:',
      '            requests:',
      '              cpu: 100m',
      '              memory: 128Mi',
      '            limits:',
      '              cpu: 500m',
      '              memory: 256Mi',
      '          livenessProbe:',
      '            httpGet:',
      '              path: /healthz',
      '              port: http',
      '          readinessProbe:',
      '            httpGet:',
      '              path: /readyz',
      '              port: http',
    ],
  },
  {
    id: 'F03',
    severity: 'medium',
    state: 'proposed',
    kind: 'Deployment',
    target: 'demo/api-gateway',
    liveYaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-gateway
  namespace: demo
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: api
          image: ghcr.io/example/api:2.0.1
          resources:
            requests:
              cpu: "32"
              memory: 64Gi`,
    diffRemoved: ['              cpu: "32"', '              memory: 64Gi'],
    diffAdded: ['              cpu: 500m', '              memory: 512Mi'],
  },
  {
    id: 'F13',
    severity: 'high',
    state: 'detected',
    kind: 'ServiceAccount',
    target: 'demo/admin-sa',
    liveYaml: `apiVersion: v1
kind: ServiceAccount
metadata:
  name: admin-sa
  namespace: demo
automountServiceAccountToken: true`,
    diffRemoved: ['    name: cluster-admin'],
    diffAdded: ['    name: namespace-editor', '  # binding narrowed to least-privilege'],
  },
  {
    id: 'F07',
    severity: 'low',
    state: 'reverted',
    kind: 'Deployment',
    target: 'demo/worker-queue',
    liveYaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker-queue
  namespace: demo
spec:
  template:
    spec:
      containers:
        - name: worker
          command: ["eecho", "consume"]`,
    diffRemoved: ['          command: ["eecho", "consume"]'],
    diffAdded: ['          command: ["echo", "consume"]'],
  },
]

// Shared diff hunk header context (the unchanged lines around each fix).
export const DIFF_CONTEXT = [
  '      containers:',
  '        - name: web',
  '          image: ghcr.io/example/web:1.4.2',
  '          ports:',
  '            - containerPort: 8080',
  '          …',
]

/**
 * Agent tool-call timeline (design mock from the ASCII sketch):
 * each entry = one tool call with its collapsible result.
 */
export const TIMELINE = [
  { tool: 'get_logs', args: 'ns=demo pod=api-gateway-9c4d tail=20', result: '3 lines · probe failed: dial tcp :8081 → connection refused', status: 'success', defaultOpen: true },
  { tool: 'get_manifest', args: 'ns=demo deploy=web-frontend', result: '42 lines loaded → sandbox copy #a1b2c3d', status: 'success', defaultOpen: true },
  { tool: 'apply_patch', args: 'ns=umbra-sandbox', result: 'patched · nothing outside the sandbox touched', status: 'success', defaultOpen: true },
  { tool: 'run_verification', args: 'ns=umbra-sandbox rule=F01', result: 'PASSED ✓ — pod Running, probes OK, no OOMKill', status: 'success', defaultOpen: true },
  { tool: 'propose_for_production', args: 'ns=demo deploy=web-frontend', result: 'queued — awaiting explicit human approval', status: 'success', defaultOpen: true },
]
