set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CERTS_DIR="$SCRIPT_DIR/../certs"

echo "Generating Certificates in: $CERTS_DIR"

mkdir -p "$CERTS_DIR"
cd "$CERTS_DIR"

echo "1. Generating CA..."
openssl req -x509 -newkey rsa:4096 -days 365 -nodes -sha256 \
  -keyout ca-key.pem -out ca-cert.pem \
  -subj "/C=IT/O=FlightData/OU=Root/CN=FlightData-Internal-CA"

echo "2. Generating Server Certificate (User Manager)..."
openssl req -newkey rsa:4096 -nodes -sha256 \
  -keyout server-key.pem -out server-req.pem \
  -subj "/C=IT/O=FlightData/OU=UserManager/CN=user-manager"

echo "subjectAltName=DNS:user-manager,DNS:localhost,IP:0.0.0.0" > server-ext.cnf

openssl x509 -req -in server-req.pem -days 120 -sha256 \
  -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial \
  -out server-cert.pem -extfile server-ext.cnf

echo "3. Generating Client Certificate (Data Collector)..."
openssl req -newkey rsa:4096 -nodes -sha256 \
  -keyout client-key.pem -out client-req.pem \
  -subj "/C=IT/O=FlightData/OU=DataCollector/CN=data-collector"

echo "subjectAltName=DNS:data-collector" > client-ext.cnf

openssl x509 -req -in client-req.pem -days 120 -sha256 \
  -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial \
  -out client-cert.pem -extfile client-ext.cnf

echo "Cleaning up temporary files..."
rm *-req.pem *-ext.cnf *.srl 2>/dev/null || true

echo "Done! Certificates are available in pkg/certs/"
