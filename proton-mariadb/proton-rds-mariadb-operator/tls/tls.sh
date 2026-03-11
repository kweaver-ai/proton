echo 01 | sudo tee ca.srl
sudo openssl genrsa -des3 -out ca-key.pem
sudo openssl req -new -x509 -days 3650 -key ca-key.pem -out ca.pem -subj "/CN=localhost"

# Server key
sudo openssl genrsa -des3 -out server-key.pem
sudo openssl req -new -key server-key.pem -out server.csr -subj "/CN=localhost"

# Server cert
echo subjectAltName = DNS:proton-rds-mariadb-operator-webhook-service.proton-rds-mariadb-operator-system.svc,IP:127.0.0.1 > extfile.cnf
sudo openssl x509 -req -days 3650 -in server.csr -CA ca.pem -CAkey ca-key.pem -out server-cert.pem -extfile extfile.cnf

sudo openssl rsa -in server-key.pem -out server-key.pem


# Client key
#sudo openssl genrsa -des3 -out client-key.pem
#sudo openssl req -new -key client-key.pem -out client.csr -subj "/CN=localhost"

# Client cert
#echo extendedKeyUsage = clientAuth > extfile.cnf
#sudo openssl x509 -req -days 3650 -in client.csr -CA ca.pem -CAkey ca-key.pem -out client-cert.pem -extfile extfile.cnf
#sudo openssl rsa -in client-key.pem -out client-key.pem

#!!!!!!!!!!!!!!!!!!!
# cat ca.pem | base64 --wrap=0
# update ValidatingWebhookConfiguration.webhooks.clientConfig.caBundle & MutatingWebhookConfiguration.webhooks.clientConfig.caBundle

