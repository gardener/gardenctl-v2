export GOOGLE_CREDENTIALS='{"client_email":"test-service-account@test-project-12345.iam.gserviceaccount.com","project_id":"test-project-12345","type":"service_account"}';
export GOOGLE_CREDENTIALS_ACCOUNT='test-service-account@test-project-12345.iam.gserviceaccount.com';
export CLOUDSDK_CORE_PROJECT='test-project-12345';
export CLOUDSDK_COMPUTE_REGION='europe';
export CLOUDSDK_CONFIG='PLACEHOLDER_CONFIG_DIR';
gcloud auth activate-service-account $GOOGLE_CREDENTIALS_ACCOUNT --key-file <(printf "%s" "$GOOGLE_CREDENTIALS");
printf 'Run the following command to revoke access credentials:\n$ eval $(gardenctl provider-env --garden test --seed seed --shoot shoot -u bash)\n';

# Run this command to configure gcloud for your shell:
# eval $(gardenctl provider-env bash)
