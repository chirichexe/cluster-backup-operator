# cluster-backup-operator

This project was made during the [Cloud Native Days Italy 2026](https://cloudnativedaysitaly.org/) conference.

Start a local Kubernetes cluster with [kind](https://kind.sigs.k8s.io/) and install the operator:

```sh
# start the controller
cd controller
make install
make run &
PID_CONTROLLER=$!

# simulate backup and restore
cd ..
./scripts/40-simulate_restore.sh

kill $PID_CONTROLLER
```

