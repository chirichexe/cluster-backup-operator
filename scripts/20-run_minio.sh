mkdir -p ${HOME}/minio/data

docker run \
  -p 9000:9000 \
  -p 9001:9001 \
  --user $(id -u):$(id -g) \
  --name minio \
  -e "MINIO_ROOT_USER=minioadmin" \
  -e "MINIO_ROOT_PASSWORD=minioadmin123" \
  -v ${HOME}/minio/data:/data \
  quay.io/minio/minio server /data --console-address ":9001"
