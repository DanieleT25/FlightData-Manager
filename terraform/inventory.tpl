[control_plane]
%{ for i, ip in control_plane_ips ~}
control-plane-${i + 1} ansible_host=${ip} ansible_user=ubuntu
%{ endfor ~}

[workers]
%{ for i, ip in worker_ips ~}
worker-${i + 1} ansible_host=${ip} ansible_user=ubuntu
%{ endfor ~}

[k8s:children]
control_plane
workers

[all:vars]
ansible_ssh_private_key_file=${ssh_private_key}
ansible_ssh_common_args=-o StrictHostKeyChecking=no
