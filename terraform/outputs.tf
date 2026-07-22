output "control_plane_ips" {
  description = "IP addresses of the control-plane node(s)"
  value       = join(" ", [for cp in multipass_instance.control_plane : cp.ipv4])
}

output "worker_ips" {
  description = "IP addresses of the worker nodes"
  value       = [for w in multipass_instance.worker : w.ipv4]
}

output "next_steps" {
  description = "Ansible commands — these run automatically inside the Gitea pipeline"
  value = {
    prerequisites = "ansible-playbook -i ~/k8s-hosts.ini ansible/00-prerequisites.yml"
    control_plane = "ansible-playbook -i ~/k8s-hosts.ini ansible/01-control-plane.yml"
    workers       = "ansible-playbook -i ~/k8s-hosts.ini ansible/02-workers.yml"
    destroy       = "tofu -chdir=terraform destroy -auto-approve"
  }
}
