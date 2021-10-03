tmp=$(mktemp -d -t runtimeclass-controller-certs-XXXXXXXXXX)
service=runtimeclass-controller
namespace=runtimeclass-controller
self="$(dirname "$(realpath "$0")")"
cat <<EOF >> $tmp/csr.conf
[req]
prompt = no
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
countryName                 = GB
stateOrProvinceName         = London
localityName               = London
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${service}
DNS.2 = ${service}.${namespace}
DNS.3 = ${service}.${namespace}.svc
EOF
openssl genrsa -out $tmp/ca.key 2048 2> /dev/null
openssl req -new -x509 -key $tmp/ca.key -out $tmp/ca.crt -config $tmp/csr.conf 2> /dev/null
openssl genrsa -out $tmp/ca.pem 2048 2> /dev/null
openssl req -new -key $tmp/ca.pem -subj "/CN=${service}.${namespace}.svc" -out $tmp/ca.csr -config $tmp/csr.conf 2> /dev/null
openssl x509 -req -in $tmp/ca.csr -CA $tmp/ca.crt -CAkey $tmp/ca.key -CAcreateserial -out $tmp/crt.pem 2> /dev/null
CA_BUNDLE=$(cat $tmp/ca.pem | base64 | tr -d '\n')
kubectl --namespace runtimeclass-controller create secret tls runtimeclass-controller-certs \
    --cert "$tmp/ca.crt" \
    --key "$tmp/ca.key" \
    -o yaml --dry-run=client
echo "---"
sed -e 's@${CA_BUNDLE}@'"$CA_BUNDLE"'@g' <"$self/runtimeclass-controller.yaml"
rm -rf $tmp