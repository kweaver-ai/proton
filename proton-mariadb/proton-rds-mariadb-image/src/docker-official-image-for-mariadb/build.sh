#docker build -t acr.aishu.cn/public/mariadb-nonroot-amd64:10.11.8 .
#docker push acr.aishu.cn/public/mariadb-nonroot-amd64:10.11.8
#docker build -t acr.aishu.cn/public/mariadb-nonroot-arm64:10.11.8 .
#docker push acr.aishu.cn/public/mariadb-nonroot-arm64:10.11.8
#docker manifest create acr.aishu.cn/public/mariadb-nonroot:10.11.8 acr.aishu.cn/public/mariadb-nonroot-amd64:10.11.8 acr.aishu.cn/public/mariadb-nonroot-arm64:10.11.8
#docker manifest push -p acr.aishu.cn/public/mariadb-nonroot:10.11.8