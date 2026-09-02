# Runbook — recovering disk from unbounded container logs

**Applies to:** any Scenario A host deployed before the log-rotation fix, most acutely
the Paladin containers.

**Why it exists:** the Paladin containers were provisioned with `debug` hardcoded and
no `logging:` block, so Docker's `json-file` driver ran with no size cap and no
rotation. One environment reached **96 GB**, roughly **2.4 GB/day** since the July
deploy. A full disk is not a degraded state — it takes the network down. The Fabric
environment lost its only orderer that way and stayed down for weeks.

The template fix stops the growth on **new** deployments. It does **not** shrink logs
that already exist: Docker keeps writing to the same file, and a `logging:` block only
takes effect when a container is recreated. This runbook is how an already-affected
host is recovered.

---

## Order of operations

Do the recovery **first**. It is independent of the template fix and the disk keeps
filling until someone runs it. Redeploying to pick up rotation can come later.

## 1. Measure before you touch anything

```bash
# Total log usage, largest first.
sudo du -sh /var/lib/docker/containers/* 2>/dev/null | sort -rh | head -20

# Map the big directories back to container names.
docker ps -a --format '{{.ID}}\t{{.Names}}' | while read id name; do
  f=$(sudo find /var/lib/docker/containers -maxdepth 2 -name "${id}*-json.log" 2>/dev/null | head -1)
  [ -n "$f" ] && echo "$(sudo du -h "$f" | cut -f1)  $name"
done | sort -rh | head -20

df -h /var/lib/docker
```

Record the numbers. Without a before-figure the recovery cannot be reported, and the
24-hour growth check in step 5 has nothing to compare against.

## 2. Truncate, do not delete

```bash
# For ONE container, by name:
f=$(sudo find /var/lib/docker/containers -maxdepth 2 \
      -name "$(docker inspect --format '{{.Id}}' <container-name>)-json.log")
sudo truncate -s 0 "$f"
```

**Truncate rather than `rm`.** The Docker daemon holds the file open; deleting it frees
nothing until the daemon releases the handle, and it leaves the container writing to a
file that no longer has a name. `truncate -s 0` reclaims the space immediately with the
container still running.

To sweep every container at once, once you have read the caveat below:

```bash
sudo find /var/lib/docker/containers -name '*-json.log' -exec truncate -s 0 {} \;
```

> **This destroys the logs.** If anything is under investigation — a stuck settlement,
> a failed join — copy what you need first (`docker logs <name> > /tmp/<name>.log`),
> and remember that copy also lands on the same disk you are trying to free.

## 3. Verify the space came back

```bash
df -h /var/lib/docker
```

If usage did not drop, something else is consuming the disk — Besu chain data and
Paladin's SQLite/LevelDB volumes grow legitimately and are **not** covered by this
runbook. Do not truncate those.

## 4. Apply the permanent cap

Rotation is a container-creation property. An existing container keeps its old,
uncapped driver until it is recreated:

```bash
cd scenario-a/samples && ./deploy-all.sh    # or re-run `cbweb3 apply` for the entity
```

After the fix, Paladin also starts at `info` rather than `debug`. To raise it for one
run, without editing any template:

```bash
PALADIN_LOG_LEVEL=debug cbweb3 apply -f <manifest>
```

Confirm the cap is actually in place on a recreated container — the point is the
*effective* configuration, not what the template says:

```bash
docker inspect --format '{{.HostConfig.LogConfig}}' <container-name>
# expected: {json-file map[max-file:5 max-size:50m]}
```

## 5. Confirm the growth rate actually fell

The fix is not proven by inspecting configuration. Measure:

```bash
# Now, then again after 24 h.
sudo du -ch /var/lib/docker/containers/*/*-json.log | tail -1
```

Expect **under 100 MB/day** across all containers at `info`, against the ~2.4 GB/day
observed at `debug`. If it is still in gigabytes, the container was not recreated, or
something is logging at debug for another reason — check the effective config from
step 4 before assuming the fix failed.

---

## What this does not cover

- **Besu chain data and Paladin state volumes.** They grow by design. Truncating them
  destroys the ledger.
- **Scenario B.** It carries the same template shape in a separate tree. Fixing it is
  a separate change, per the scenario-isolation rule.
- **Log shipping.** Capping at 250 MB per service (50m x 5) bounds the disk and
  therefore bounds how far back local history goes. Anywhere that needs longer
  retention needs a log shipper, which is its own piece of work.
