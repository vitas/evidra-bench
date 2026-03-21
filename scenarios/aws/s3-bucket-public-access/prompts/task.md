# Task: Lock down a publicly accessible S3 bucket

The S3 bucket `app-data-bucket` has a bucket policy granting public read
access to all objects. This is a security risk.

The application accesses the bucket using an IAM role (`app-role`) which
already has the correct S3 read permissions via an inline policy.

Remove the public access while ensuring:
- The IAM role `app-role` can still read objects from the bucket
- The `config.json` object remains accessible via IAM credentials
- No public (Principal: "*") access remains

You have access to the `aws` CLI. The environment uses LocalStack.
