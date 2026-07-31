# The monitoring stack itself is installed by the aws-monitoring workflow with
# Helm, not by OpenTofu: it lives inside the cluster, and Kubernetes objects
# belong to kubectl and Helm rather than to the infrastructure state. What is
# needed here is the one value the workflow cannot invent — Grafana's admin
# password.
#
# It is generated rather than asked for, the same way the RDS one is, so no
# human ever handles it and no manual parameter has to be created before the
# first run. Like every generated secret it is written to the state in clear
# text, which is why that bucket is private, encrypted and versioned.
resource "random_password" "grafana" {
  length = 24
  # Grafana is reached through a browser and a terminal, so the alphabet stays
  # free of characters that need escaping in a shell or a URL.
  override_special = "-_.~"
}

resource "aws_ssm_parameter" "grafana_password" {
  name  = "${local.ssm_prefix}/grafana/password"
  type  = "SecureString"
  value = random_password.grafana.result
}
