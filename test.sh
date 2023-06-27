go test -timeout 30s -run ^TestFunc$ k8s.io/kubernetes/cmd/kubeadm -count=1 2>&1 1>/dev/null
if [ $? -eq 0 ]
then
    echo "test: ok"
    exit 0
else
    echo "test: fail"
    exit 1
fi
