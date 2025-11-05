export GOOGLE_CREDENTIALS_ACCOUNT=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-client_email.txt');
export CLOUDSDK_CORE_PROJECT=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-project_id.txt');
export CLOUDSDK_COMPUTE_REGION=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-region.txt');
export CLOUDSDK_CONFIG='PLACEHOLDER_CONFIG_DIR';
gcloud auth activate-service-account --key-file 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-credentials.txt' -- "$GOOGLE_CREDENTIALS_ACCOUNT";
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-client_email.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-credentials.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-project_id.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-region.txt';
printf 'Run the following command to revoke access credentials:\n$ eval $(gardenctl provider-env --garden test --seed seed --shoot shoot -u bash)\n';

# Run this command to configure gcloud for your shell:
# eval $(gardenctl provider-env bash)
