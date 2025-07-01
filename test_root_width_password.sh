export SSH_TEST_HOST="172.30.1.102"
export SSH_TEST_USER="root"
export SSH_TEST_PASSWORD="Def@u1tpwd"
export SSH_TEST_PORT=22
go test -v github.com/mensylisir/kubexm/pkg/connector
