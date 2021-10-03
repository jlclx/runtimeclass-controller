tmp=$(mktemp -d -t runtimeclass-controller-certs-XXXXXXXXXX)
self="$(dirname "$(realpath "$0")")"
openssl req -nodes -new -x509 -keyout $tmp/ca-key.pem -out $tmp/ca-crt.pem -subj "/CN=runtimeclass-controller.runtimeclass-controller.svc" 2>/dev/null
openssl genrsa -out $tmp/key.pem 2048 2>/dev/null
openssl req -new -key $tmp/key.pem -subj "/CN=runtimeclass-controller.runtimeclass-controller.svc" 2>/dev/null | \
openssl x509 -req -CA $tmp/ca-crt.pem -CAkey $tmp/ca-key.pem -CAcreateserial -out $tmp/crt.pem 2>/dev/null
CA_BUNDLE=$(cat $tmp/ca-crt.pem | base64 | tr -d '\n')
kubectl --namespace runtimeclass-controller create secret tls runtimeclass-controller-certs \
    --cert "$tmp/crt.pem" \
    --key "$tmp/key.pem" \
    -o yaml --dry-run=client
echo "---"
sed -e 's@${CA_BUNDLE}@'"$CA_BUNDLE"'@g' <"$self/runtimeclass-controller.yaml"
rm -rf $tmp