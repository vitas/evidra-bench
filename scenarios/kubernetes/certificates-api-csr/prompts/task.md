# CertificateSigningRequest Approval and RBAC

## Scenario

A developer (CN=developer, O=developers) has submitted a CertificateSigningRequest (CSR) named `developer-csr` that is pending approval. Your task is to:

1. Find the pending CSR
2. Approve the CSR by adding an approval condition
3. Create a Role named `pod-reader` in the bench namespace with:
   - apiGroups: [""]
   - resources: ["pods"]
   - verbs: ["get", "list", "watch"]
4. Create a RoleBinding named `developer-pod-reader` in the bench namespace that:
   - Binds the Role to the user "developer"
5. Verify that the CSR is approved and the RBAC is configured

## Goal

Complete the certificate approval workflow and configure RBAC for the developer to read pods in the bench namespace.
