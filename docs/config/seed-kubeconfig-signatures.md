# Seed kubeconfig signatures

For a non-managed seed, gardenctl loads the kubeconfig from the `garden/<seed>.login` Secret in the virtual Garden. By default, it verifies the signature before parsing the kubeconfig. Managed seeds use the shoot kubeconfig flow.

## Configure a trusted public key

Obtain the ECDSA P-256 public key from your landscape administrator through an authenticated channel independent of the virtual Garden. Add the PEM `PUBLIC KEY` block (SubjectPublicKeyInfo) to the matching Garden by identity or alias. If you have the key in a file, use `--file`.

A trusted key decides which kubeconfig gardenctl loads for the Garden's non-managed seeds. A kubeconfig can name commands that kubectl and gardenctl run on your machine to authenticate, local files to read, and the server that receives the credentials. Add a key only after confirming with the landscape administrator that it is theirs.

```sh
gardenctl config add-seed-key landscape-dev --file public-key.pem
```

You can also use `--key` to include the PEM public key in a single command for easy sharing. Enclose the complete PEM block in single quotes and keep its line breaks.

```sh
gardenctl config add-seed-key landscape-dev --key '-----BEGIN PUBLIC KEY-----
<public key from your landscape administrator>
-----END PUBLIC KEY-----'
```

Replace the placeholder with the contents of your public key file between the PEM header and footer. Run `gardenctl config add-seed-key --help` to see both input options.

The command appends the key to `seedKubeconfigPublicKeys` in the local configuration file (by default `~/.garden/gardenctl-v2.yaml`). You can also edit this list directly. A signature must verify with at least one key in the Garden's list.

```yaml
gardens:
  - identity: landscape-dev
    kubeconfig: /path/to/garden-kubeconfig.yaml
    seedKubeconfigPublicKeys:
      - |
        -----BEGIN PUBLIC KEY-----
        <public key from your landscape administrator>
        -----END PUBLIC KEY-----
```

## Opt out for a Garden

Use `insecureSkipSeedKubeconfigSignatureVerification: true` in the local Garden entry for environments where signing is unnecessary, such as a test environment, or where signing is unavailable or the landscape administrator has chosen to operate without it.

> [!CAUTION]
> This disables protection against kubeconfig manipulation. Anyone who can modify the seed login Secret can replace the kubeconfig gardenctl loads.

```yaml
gardens:
  - identity: landscape-test
    kubeconfig: /path/to/garden-kubeconfig.yaml
    insecureSkipSeedKubeconfigSignatureVerification: true
```

Choose either trusted `seedKubeconfigPublicKeys` or the opt-out for each Garden. Gardenctl rejects a configuration that enables both. With the opt-out, the `<seed>.login` Secret only needs the `kubeconfig` data entry.

## Sign a kubeconfig

Generate the kubeconfig from an approved source and sign it in a trusted environment. Use a distinct ECDSA P-256 key for each landscape. The manifest names the seed but not the landscape, so a key shared between landscapes would make a kubeconfig signed for one landscape acceptable for a seed of the same name in another. Keep the private key and signing permissions separate from the publishing pipeline.

To generate a file-based key pair, run these commands once in a private directory. Distribute `public-key.pem` to users.

```sh
(umask 077 && openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:prime256v1 -pkeyopt ec_param_enc:named_curve -out signing-key.pem)
openssl pkey -in signing-key.pem -pubout -out public-key.pem
```

Gardenctl accepts only public keys that identify the curve by name. The LibreSSL build that provides `openssl` on macOS writes explicit curve parameters unless `ec_param_enc:named_curve` is set. It also leaves the private key file readable by other users unless the umask prevents it.

Run `openssl dgst -sha256 kubeconfig.yaml` and copy the lowercase digest into `manifest.json`, keeping the `sha256:` prefix. Set `seedName` to the seed resource name. Use the four fields shown below.

```json
{
  "apiVersion": "gardenctl.gardener.cloud/v1alpha1",
  "kind": "SeedKubeconfigSignature",
  "seedName": "seed-a",
  "kubeconfigHash": "sha256:<lowercase hexadecimal digest>"
}
```

Sign the exact bytes of `manifest.json` with ECDSA P-256 and SHA-256. Store the signature as binary ASN.1 DER. For a file-based key, sign and verify with OpenSSL.

```sh
openssl dgst -sha256 -sign signing-key.pem -out manifest.sig manifest.json
openssl dgst -sha256 -verify public-key.pem -signature manifest.sig manifest.json
```

## Publish and verify the Secret

Give the publishing pipeline the three files below. Deploy them together as the `<seed>.login` Secret in the `garden` namespace of the virtual Garden, preserving the exact file bytes. Re-sign and publish whenever an approved kubeconfig changes.

| Secret data key | Content after Kubernetes base64 decoding |
|---|---|
| `kubeconfig` | `kubeconfig.yaml` |
| `kubeconfig-signing-manifest` | `manifest.json` |
| `kubeconfig-signature` | `manifest.sig` |

Verify from a client configured with the trusted public key.

```sh
gardenctl kubeconfig --garden landscape-dev --seed seed-a --access-level=auto >/dev/null
```

Signatures have no expiry. Gardenctl accepts older kubeconfigs for a seed while their signing key remains trusted, even after a newer kubeconfig is published. Existing shells keep using their loaded kubeconfig until the session is refreshed.
