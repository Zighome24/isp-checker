# Network Troubleshooting for ISP-Checker on Raspberry Pi

## Understanding `net.ipv4.ping_group_range`

The `net.ipv4.ping_group_range` sysctl parameter controls which Linux group IDs (GIDs) are allowed to use ICMP sockets (ping) without root privileges. This is a security feature.

### How It Works

- **Format**: `net.ipv4.ping_group_range = min_gid max_gid`
- **Default**: Often `1 0` (meaning no groups can ping without root)
- **Unrestricted**: `0 2147483647` (all groups can ping)

When a container runs, it typically gets a non-root user. If that user's group ID falls outside the `ping_group_range`, ICMP ping operations will fail.

## Commands to Check Current Settings

### 1. Check Current Value

```bash
# Read the current sysctl value
sysctl net.ipv4.ping_group_range

# Or read from proc filesystem
cat /proc/sys/net/ipv4/ping_group_range
```

**Example output:**
```
net.ipv4.ping_group_range = 1 0
```
This means no groups can ping (1 to 0 is an empty range).

### 2. Check Your User's Group ID

```bash
# Check your primary group ID
id -g

# Check all group IDs for your user
id

# For the systemd service user (usually root)
id root
```

### 3. Test Ping as Your Container User

```bash
# Test ping capability directly
sudo -u $(whoami) ping -c 1 8.8.8.8
```

## Commands to Update Settings

### Temporary Change (Until Reboot)

```bash
# Allow all groups to use ping
sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"

# Verify the change
sysctl net.ipv4.ping_group_range
```

### Permanent Change (Survives Reboot)

#### Method 1: Create a sysctl.d file (Recommended)

```bash
# Create a new sysctl configuration file
echo "net.ipv4.ping_group_range = 0 2147483647" | sudo tee /etc/sysctl.d/99-ping.conf

# Apply the configuration
sudo sysctl --system

# Verify
sysctl net.ipv4.ping_group_range
```

#### Method 2: Edit sysctl.conf directly

```bash
# Append to main sysctl.conf
echo "net.ipv4.ping_group_range = 0 2147483647" | sudo tee -a /etc/sysctl.conf

# Apply
sudo sysctl -p
```

### Targeted Change (Specific Group Only)

If you prefer a more secure approach, allow only specific groups:

```bash
# Find the group ID of the podman user (or whatever user runs the container)
GROUP_ID=$(id -g podman 2>/dev/null || id -g)
echo "net.ipv4.ping_group_range = $GROUP_ID $GROUP_ID" | sudo tee /etc/sysctl.d/99-ping.conf
sudo sysctl --system
```

## Container-Specific Considerations

### Running Rootless

If running Podman in rootless mode, the container uses user namespaces:

```bash
# Check your user's subordinate GID range
cat /etc/subgid

# Example output:
# username:100000:65536
# This means your container GIDs map to 100000-165535 on the host
