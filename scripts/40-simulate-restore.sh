#!/usr/bin/env bash

# Abilita il fail-fast per una gestione robusta degli errori in stile engineering
set -euo pipefail

# Definizione delle costanti di ambiente
NS_TARGET="nginx"
BUCKET_NAME="workshop-backups"
BACKUP_NAME="backup-sample"
BACKUP_FILE="tests/${BACKUP_NAME}.json"
CRD_MANIFEST="controller/config/samples/operators_v1alpha1_restore.yaml"

# Colori per l'output di logging
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

log_info() {
  echo -e "${GREEN}[INFO] $(date +'%Y-%m-%d %H:%M:%S') - $1${NC}"
}

log_error() {
  echo -e "${RED}[ERROR] $(date +'%Y-%m-%d %H:%M:%S') - $1${NC}" >&2
}

# 1. Validazione della gerarchia dei file e dei prerequisiti locali
log_info "Verifica della consistenza della directory corrente..."
if [[ ! -d "controller" || ! -d "scripts" || ! -d "tests" ]]; then
  log_error "Esecuzione fallita: lo script deve essere lanciato dalla root del progetto."
  exit 1
fi

if [[ ! -f "$BACKUP_FILE" ]]; then
  log_info "File $BACKUP_FILE non trovato. Generazione automatica del payload unstructured..."
  cat <<EOF >"$BACKUP_FILE"
[
  {
    "apiVersion": "v1",
    "kind": "ConfigMap",
    "metadata": {
      "name": "restore-test-data",
      "namespace": "${NS_TARGET}"
    },
    "data": {
      "status": "successfully-restored",
      "cluster": "kind-workshop",
      "timestamp": "$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
    }
  }
]
EOF
fi

# 2. Controllo dello stato del container MinIO
log_info "Verifica dello stato del container MinIO..."
if ! docker ps --format '{{.Names}}' | grep -q '^minio$'; then
  log_error "Il container 'minio' non è in esecuzione. Avvia scripts/20-run_minio.sh prima di procedere."
  exit 1
fi

# 3. Configurazione dell'Object Storage (MinIO Client interno)
log_info "Sincronizzazione dell'alias mc e creazione del bucket se inesistente..."
docker exec minio mc alias set local http://localhost:9000 minioadmin minioadmin123 >/dev/null

if ! docker exec minio mc ls local/ | grep -q "${BUCKET_NAME}"; then
  docker exec minio mc mb local/${BUCKET_NAME}
fi

# 4. Caricamento del payload JSON (Simulazione dell'esistenza del Backup)
log_info "Trasferimento del file di backup nell'object storage..."
docker cp "$BACKUP_FILE" minio:/tmp/${BACKUP_NAME}.json
docker exec minio mc cp /tmp/${BACKUP_NAME}.json local/${BUCKET_NAME}/${BACKUP_NAME}.json >/dev/null

# 5. Preparazione del cluster Kubernetes (Kind)
log_info "Configurazione del namespace di target nel cluster..."
kubectl create ns ${NS_TARGET} --dry-run=client -o yaml | kubectl apply -f -

# 6. Generazione del manifesto dichiarativo per la CRD Restore
log_info "Generazione del manifesto dichiarativo per la risorsa Restore..."
mkdir -p "$(dirname "$CRD_MANIFEST")"
cat <<EOF >"$CRD_MANIFEST"
apiVersion: operators.com/v1alpha1
kind: Restore
metadata:
  name: restore-sample
  namespace: ${NS_TARGET}
spec:
  backupName: ${BACKUP_NAME}
  storageBucket: ${BUCKET_NAME}
EOF

# Pulizia di vecchie istanze per forzare un Reconcile Loop pulito
kubectl delete -f "$CRD_MANIFEST" --ignore-not-found=true --grace-period=0

# 7. Applicazione della risorsa nel cluster
log_info "Applicazione del Custom Resource Object al cluster..."
kubectl apply -f "$CRD_MANIFEST"

# 8. Loop di validazione asincrona dello stato del reconcile
log_info "In attesa della riconciliazione da parte del controller..."
TIMEOUT=30
ELAPSED=0
STATUS="Unknown"

while [ $ELAPSED -lt $TIMEOUT ]; do
  # Estrazione sicura del campo status tramite jsonpath
  STATUS=$(kubectl get restore restore-sample -n ${NS_TARGET} -o jsonpath='{.status.status}' 2>/dev/null || echo "Unknown")

  if [ "$STATUS" == "Completed" ]; then
    log_info "Reconciliation completata con successo!"
    break
  elif [ "$STATUS" == "Failed" ]; then
    MSG=$(kubectl get restore restore-sample -n ${NS_TARGET} -o jsonpath='{.status.message}' 2>/dev/null || echo "N/A")
    log_error "Il controller ha marcato il restore come fallito: $MSG"
    exit 1
  fi

  sleep 2
  ELAPSED=$((ELAPSED + 2))
done

if [ "$STATUS" != "Completed" ]; then
  log_error "Timeout raggiunto. Il controller non ha risposto entro $TIMEOUT secondi."
  exit 1
fi

# 9. Controllo di consistenza finale dei dati iniettati
log_info "Validazione della risorsa ripristinata nel cluster:"
kubectl get configmap restore-test-data -n ${NS_TARGET} -o yaml

log_info "Simulazione completata con successo ed ambiente consistente."
