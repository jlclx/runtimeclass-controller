tmp=$(mktemp -d -t runtimeclass-controller-certs-XXXXXXXXXX)
self="$(dirname "$(realpath "$0")")"
openssl req -nodes -new -x509 -keyout $tmp/ca.key -out $tmp/ca.crt -subj "/CN=RuntimeClass Controller" 2>/dev/null
openssl genrsa -out $tmp/tls.key 2048 2>/dev/null
openssl req -new -key $tmp/tls.key -subj "/CN=runtimeclass-controller.runtimeclass-controller.svc" 2>/dev/null | \
openssl x509 -req -CA $tmp/ca.crt -CAkey $tmp/ca.key -CAcreateserial -out $tmp/tls.crt 2>/dev/null
CA_BUNDLE=$(cat $tmp/ca.crt | base64 | tr -d '\n')
kubectl --namespace runtimeclass-controller create secret tls runtimeclass-controller-certs \
    --cert "$tmp/tls.crt" \
    --key "$tmp/tls.key" \
    -o yaml --dry-run=client
echo "---"
sed -e 's@${CA_BUNDLE}@'"$CA_BUNDLE"'@g' <"$self/runtimeclass-controller.yaml"

rm -rf $tmp