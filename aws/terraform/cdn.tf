# Static frontend on S3, served through CloudFront.
#
# The Svelte application is a Vite build — a few hundred kilobytes of HTML, CSS
# and JavaScript with no server-side logic — so serving it from a container is
# paying for compute to hand out files. A CDN does it closer to the user, and
# both services stay inside the free tier at this size.
#
# What this deployment cannot do, and it is worth being precise about: the
# application served from CloudFront will render but will not work. The Svelte
# code calls its API with relative paths — `fetch('/api/interests')` — which
# resolve against whatever origin the browser loaded the page from. Under
# nginx, frontend and API share an origin and that works. Under CloudFront the
# call would hit the CDN, which only knows about the S3 bucket.
#
# Fixing it would mean giving the API a public address for CloudFront to use as
# a second origin, and that requires a load balancer — which this account's plan
# refuses to create, as verified from the console:
#
#   OperationNotPermittedException: This AWS account currently does not support
#   creating load balancers.
#
# So the frontend Deployment is deliberately kept in the cluster as well. It is
# not a duplicate by oversight: nginx serving both frontend and API on one
# origin is the path that actually works, reachable with a forwarded port, while
# this distribution demonstrates the CDN tier of the target architecture.

resource "random_id" "frontend_bucket" {
  byte_length = 4
}

# Bucket names are globally unique. A random suffix is used rather than the
# account id, which must not appear in a public workflow log.
resource "aws_s3_bucket" "frontend" {
  bucket = "${local.name}-frontend-${random_id.frontend_bucket.hex}"

  # The bucket holds build output and nothing irreplaceable, and every teardown
  # must succeed without a manual emptying step first.
  force_destroy = true

  tags = { Name = "${local.name}-frontend" }
}

# The bucket is never public. CloudFront reaches it through an Origin Access
# Control, so the only way to the files is through the distribution — which also
# means the HTTPS redirect and the caching cannot be bypassed.
resource "aws_s3_bucket_public_access_block" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  block_public_acls       = true
  block_public_policy     = false # the policy below grants CloudFront, not the public
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_cloudfront_origin_access_control" "frontend" {
  name                              = "${local.name}-frontend"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

data "aws_cloudfront_cache_policy" "optimized" {
  name = "Managed-CachingOptimized"
}

resource "aws_cloudfront_distribution" "frontend" {
  enabled             = true
  is_ipv6_enabled     = true
  default_root_object = "index.html"
  comment             = "${local.name} static frontend"

  # Edge locations in Europe and North America only. The full set would serve
  # Asia and South America faster at a higher price, for an audience this
  # project does not have.
  price_class = "PriceClass_100"

  origin {
    domain_name              = aws_s3_bucket.frontend.bucket_regional_domain_name
    origin_id                = "s3-frontend"
    origin_access_control_id = aws_cloudfront_origin_access_control.frontend.id
  }

  default_cache_behavior {
    target_origin_id       = "s3-frontend"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    cache_policy_id        = data.aws_cloudfront_cache_policy.optimized.id
    compress               = true
  }

  # A single-page application routes on the client, so the bucket has no object
  # at /login or /dashboard. Without these rules S3 would answer 403 or 404 and
  # the page would break on reload or on a direct link; returning index.html
  # with a 200 lets the application resolve the route itself.
  dynamic "custom_error_response" {
    for_each = [403, 404]
    content {
      error_code         = custom_error_response.value
      response_code      = 200
      response_page_path = "/index.html"
    }
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  # The default *.cloudfront.net certificate. A custom domain would need one
  # from ACM in us-east-1, and there is no domain to attach.
  viewer_certificate {
    cloudfront_default_certificate = true
  }

  tags = { Name = "${local.name}-frontend" }
}

# Grants read access to this distribution alone — not to the public, and not to
# any other distribution in any account.
data "aws_iam_policy_document" "frontend_bucket" {
  statement {
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.frontend.arn}/*"]

    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.frontend.arn]
    }
  }
}

resource "aws_s3_bucket_policy" "frontend" {
  bucket = aws_s3_bucket.frontend.id
  policy = data.aws_iam_policy_document.frontend_bucket.json

  depends_on = [aws_s3_bucket_public_access_block.frontend]
}

# Published the same way as every other runtime value, so the deploy workflow
# reads them from Parameter Store instead of receiving them from OpenTofu.
resource "aws_ssm_parameter" "frontend_bucket" {
  name  = "${local.ssm_prefix}/frontend/bucket"
  type  = "String"
  value = aws_s3_bucket.frontend.id
}

resource "aws_ssm_parameter" "frontend_distribution" {
  name  = "${local.ssm_prefix}/frontend/distribution"
  type  = "String"
  value = aws_cloudfront_distribution.frontend.id
}
