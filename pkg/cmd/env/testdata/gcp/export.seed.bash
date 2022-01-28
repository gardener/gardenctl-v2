export GOOGLE_CREDENTIALS='{"client_email":"test@example.org","project_id":"test"}';
export GOOGLE_CREDENTIALS_ACCOUNT='test@example.org';
export CLOUDSDK_CORE_PROJECT='test';
export CLOUDSDK_COMPUTE_REGION='europe';
export CLOUDSDK_CONFIG='%[1]s/.config/gcloud';
gcloud auth activate-service-account $GOOGLE_CREDENTIALS_ACCOUNT --key-file <(printf "%%s" "$GOOGLE_CREDENTIALS");
printf 'Run the following command to revoke access credentials:\n$ eval $(gardenctl provider-env --garden test --seed seed --shoot shoot -u bash)\n';

# Run this command to configure gcloud for your shell:
# eval $(gardenctl provider-env bash)
