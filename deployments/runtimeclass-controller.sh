tmp=$(mktemp -d -t runtimeclass-controller-certs-XXXXXXXXXX)
service=runtimeclass-controller
namespace=runtimeclass-controller
self="$(dirname "$(realpath "$0")")"
cat <<EOF >> $tmp/san.conf
[req]
prompt = no
req_extensions = req_ext
distinguished_name = req_distinguished_name
[req_distinguished_name]
countryName                 = GB
stateOrProvinceName         = London
localityName               = London
[ req_ext ]
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${service}
DNS.2 = ${service}.${namespace}
DNS.3 = ${service}.${namespace}.svc
EOF
openssl req -nodes -new -x509 -keyout $tmp/ca.key -out $tmp/ca.crt -subj "/CN=Runtimeclass Controller CA" 2> /dev/null
openssl genrsa -out $tmp/tls.key 2048 2> /dev/null
openssl req -new -key $tmp/tls.key -config $tmp/san.conf -out $tmp/tls.csr 2> /dev/null
openssl x509 -req -in $tmp/tls.csr -CA $tmp/ca.crt -CAkey $tmp/ca.key -CAcreateserial -out $tmp/tls.crt -extensions req_ext -extfile $tmp/san.conf 2> /dev/null
CA_BUNDLE=$(cat $tmp/ca.crt | base64 | tr -d '\n')
kubectl --namespace runtimeclass-controller create secret tls runtimeclass-controller-certs \
    --cert "$tmp/tls.crt" \
    --key "$tmp/tls.key" \
    -o yaml --dry-run=client
echo "---"
sed -e 's@${CA_BUNDLE}@'"$CA_BUNDLE"'@g' <"$self/runtimeclass-controller.yaml"
rm -rf $tmp