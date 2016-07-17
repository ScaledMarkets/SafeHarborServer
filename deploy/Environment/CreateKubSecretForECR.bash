# Create AWS docker config as a kub secret.
# Ref: https://blog.redspread.com/2016/02/09/Using-AWS-EC2-Container-Registry-with-K8S/

# Get the docker login command for our AWS Docker Image Registry, and then log into it:
aws ecr get-login | sh -
# aws ecr get-login gives result,
#	docker login -u AWS -p CiBwm0YaISJ... -e none https://[accountnum].dkr.ecr.us-east-1.amazonaws.com

# Create a kub file that defines our docker config as a kub secret:
cat > /tmp/image-pull-secret.yaml << EOF 
apiVersion: v1     
kind: Secret
metadata:
  name: scaledmarkets_registrykey
data:
  .dockerconfigjson: $(cat ~/.docker/config.json | base64 -w 0)
type: kubernetes.io/dockerconfigjson
EOF

# Apply the kub file to actually create the kub secret:
kubectl create -f /tmp/image-pull-secret.yaml
#kubectl replace -f /tmp/image-pull-secret.yaml
