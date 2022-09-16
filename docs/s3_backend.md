## Feature: s3 backend

If the description pasted into the graph annotation is too long, it will be omitted and broken up in the normal simple setup.
Then, upload the description of the Full version to S3, and add the url of a simple viewer where the Full version can be viewed on the graph annotation.


```hcl
prepalert {
  required_version = ">=v0.2.0"
  sqs_queue_name   = "prepalert"
  service          = "prod"

  s3_backend {
    # Upload information about alerts to S3 and set up a simplified view of the uploaded information.
    bucket_name                 = "prepalert-infomation"
    object_key_prefix           = "alerts/"
    viewer_base_url             = "<your prepalert http server address>"
  }
}

rule "simple" {
  alert {
    any = true
  }
  infomation = <<EOF
How do you respond to alerts?
Describe information about your alert response here.
EOF
}
```

If you want to restrict viewing by Google OIDC on the simple viewer, change as follows.

```diff
--- hclconfig/testdata/s3_backend/config.hcl   before
+++ hclconfig/testdata/s3_backend/config.hcl   after
@@ -8,6 +8,9 @@
     bucket_name                 = "prepalert-infomation"
     object_key_prefix           = "alerts/"
     viewer_base_url             = "<your prepalert http server address>"
+    viewer_google_client_id     = env("GOOGLE_CLIENT_ID", "")
+    viewer_google_client_secret = env("GOOGLE_CLIENT_SECRET", "")
+    viewer_session_encrypt_key  = env("SESSION_ENCRYPT_KEY", "")
   }
 }
```
