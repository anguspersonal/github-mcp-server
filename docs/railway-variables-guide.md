# Railway Variables Guide

Understanding Railway's variable system is important for configuring your deployment correctly.

## Types of Variables in Railway

Railway has three main types of variables:

### 1. Service Variables
**Location**: Service → Variables tab

- **Scope**: Specific to one service
- **Available**: During Docker builds AND at runtime
- **Use for**: Service-specific configuration, secrets, build arguments
- **Example**: `RAILWAY_SERVICE_ID`, `GITHUB_APP_ID`

**When to use**: 
- Variables needed during Docker build (like `RAILWAY_SERVICE_ID` for cache mounts)
- Service-specific secrets
- Configuration unique to this service

### 2. Shared Variables
**Location**: Project Settings → Shared Variables

- **Scope**: Can be shared across multiple services in the project
- **Available**: Primarily at runtime (may not be available during builds)
- **Use for**: Common configuration shared across services
- **Example**: Database connection strings shared by multiple services

**When to use**:
- Configuration needed by multiple services
- Values that should be consistent across services
- **Note**: Not recommended for build-time variables

### 3. Railway-Provided Variables
**Location**: Automatically provided by Railway

- **Scope**: Automatically available to all services
- **Available**: During builds and runtime
- **Examples**: 
  - `RAILWAY_PUBLIC_DOMAIN`
  - `RAILWAY_PRIVATE_DOMAIN`
  - `RAILWAY_PROJECT_NAME`
  - `RAILWAY_SERVICE_NAME`
  - `RAILWAY_PROJECT_ID`
  - `RAILWAY_ENVIRONMENT_ID`
  - `PORT` (automatically set by Railway)

**Note**: Railway does NOT automatically provide `RAILWAY_SERVICE_ID` - you must set it manually as a Service Variable.

## For This Project


### Required Service Variables

Set these in **Service → Variables tab** (not Shared Variables):

1. **`RAILWAY_SERVICE_ID`** (for Docker cache mounts)
   - Get it: Service → Press `Ctrl+K` → "Copy Service ID"
   - Set it: Service → Variables → New Variable
   - **Why Service Variable?**: Needed during Docker build

2. **`GITHUB_APP_ID`** (for GitHub App authentication)
   - From your GitHub App settings
   - Set it: Service → Variables → New Variable

3. **`GITHUB_APP_PRIVATE_KEY_B64`** (for GitHub App authentication)
   - Base64-encoded private key from your GitHub App
   - Set it: Service → Variables → New Variable

4. **`GITHUB_MCP_TOKEN_MAP`** (for token mapping)
   - JSON mapping of MCP tokens to installation IDs
   - Set it: Service → Variables → New Variable

### Optional Service Variables

- `GITHUB_HOST` (for GitHub Enterprise)
- `GITHUB_TOOLSETS` (to customize available toolsets)
- `LOG_LEVEL` (for logging verbosity)

## Quick Reference

| Variable Type | Location | Build Time? | Runtime? | Use For |
|--------------|----------|-------------|----------|---------|
| Service Variable | Service → Variables | ✅ Yes | ✅ Yes | Build args, service secrets |
| Shared Variable | Project Settings | ❌ Usually No | ✅ Yes | Cross-service config |
| Railway-Provided | Automatic | ✅ Yes | ✅ Yes | Railway metadata |

## Common Mistakes

1. **Setting `RAILWAY_SERVICE_ID` as Shared Variable**
   - ❌ Wrong: Project Settings → Shared Variables
   - ✅ Correct: Service → Variables tab

2. **Expecting Railway to auto-provide `RAILWAY_SERVICE_ID`**
   - Railway does NOT automatically provide this
   - You must manually set it as a Service Variable

3. **Using Shared Variables for build-time needs**
   - Shared variables may not be available during Docker builds
   - Use Service Variables for anything needed during build

## Verifying Your Setup

1. Check Service Variables:
   - Go to Service → Variables tab
   - Verify `RAILWAY_SERVICE_ID` is listed under "Service Variables" (not "Shared Variables")

2. Check Build Logs:
   - After setting `RAILWAY_SERVICE_ID`, trigger a new build
   - Check build logs - cache mount errors should be resolved

3. Verify Runtime:
   - After deployment, check service logs
   - Service should start without authentication errors

