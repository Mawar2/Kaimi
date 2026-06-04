# Setup Scripts

## Environment Setup

These scripts automatically fetch API keys from GCP Secret Manager and create your local `.env` file.

### Prerequisites

- `gcloud` CLI installed
- Authenticated with `gcloud auth login`
- Access to the `kaimi-seeker` GCP project
- Secret Manager Secret Accessor role

### Usage

**On Windows:**
```cmd
scripts\setup-env.bat
```

**On Mac/Linux:**
```bash
chmod +x scripts/setup-env.sh
./scripts/setup-env.sh
```

### What It Does

1. ✅ Verifies you're authenticated with GCP
2. ✅ Sets the project to `kaimi-seeker`
3. ✅ Fetches `samgov-api-key` from Secret Manager
4. ✅ Fetches `google-ai-studio-api-key` from Secret Manager
5. ✅ Creates a `.env` file with the secrets

### After Running

The `.env` file will be created in your project root with all necessary API keys.

**Note:** The `.env` file is in `.gitignore` and will never be committed to the repository.
