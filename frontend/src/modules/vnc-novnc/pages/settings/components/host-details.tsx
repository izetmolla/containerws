import {
  Box,
  CheckCircle2,
  Cpu,
  HardDrive,
  Info,
  Monitor,
  Server,
  XCircle,
} from "lucide-react"

import { cn } from "@/lib/utils"

import type { HostPlan, StatusReport } from "../api"

type HostDetailsProps = {
  plan: HostPlan
  status: StatusReport
}

function DetailItem({
  label,
  value,
  mono,
}: {
  label: string
  value: string
  mono?: boolean
}) {
  if (!value) return null
  return (
    <div className="space-y-1">
      <dt className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
        {label}
      </dt>
      <dd
        className={cn(
          "text-sm text-foreground",
          mono && "font-mono text-[13px]"
        )}
      >
        {value}
      </dd>
    </div>
  )
}

function environmentLabel(plan: HostPlan) {
  if (plan.is_container) return "Container"
  if (plan.is_vm) return "Virtual machine"
  return "Bare metal"
}

export function HostDetails({ plan, status }: HostDetailsProps) {
  const device =
    plan.device_type ||
    (plan.is_container ? "container" : plan.is_vm ? "vm" : "host")

  return (
    <div className="space-y-6">
      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <Server className="size-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
            Machine
          </h2>
        </div>
        <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <DetailItem label="Hostname" value={plan.hostname} mono />
          <DetailItem
            label="Architecture"
            value={plan.arch || plan.platform}
            mono
          />
          <DetailItem label="Device" value={device} />
          <DetailItem label="Environment" value={environmentLabel(plan)} />
          <DetailItem label="Virtualization" value={plan.virtualization} />
          <DetailItem label="Kernel" value={plan.kernel} mono />
        </dl>
      </section>

      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <Monitor className="size-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
            Operating system
          </h2>
        </div>
        <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <DetailItem
            label="Distribution"
            value={
              [plan.distro || plan.os, plan.distro_version]
                .filter(Boolean)
                .join(" ")
            }
          />
          <DetailItem label="Distro ID" value={plan.distro_id} mono />
          <DetailItem label="OS" value={plan.os} />
          <DetailItem
            label="Package family"
            value={plan.package_family || "unknown"}
          />
          <DetailItem
            label="Package manager"
            value={plan.package_manager}
            mono
          />
          <div className="space-y-1">
            <dt className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
              Setup support
            </dt>
            <dd>
              {plan.supported ? (
                <span className="inline-flex items-center gap-1 text-sm text-emerald-700 dark:text-emerald-300">
                  <CheckCircle2 className="size-3.5" />
                  Supported
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 text-sm text-amber-700 dark:text-amber-300">
                  <XCircle className="size-3.5" />
                  Unsupported
                </span>
              )}
            </dd>
          </div>
        </dl>
      </section>

      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <HardDrive className="size-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
            Component status
          </h2>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          {Object.entries(status.binaries ?? {}).map(([name, ok]) => (
            <div
              key={name}
              className="flex items-center justify-between border-b border-border/60 py-2"
            >
              <span className="flex items-center gap-2 font-mono text-sm">
                <Cpu className="size-3.5 text-muted-foreground" />
                {name}
              </span>
              {ok ? (
                <span className="inline-flex items-center gap-1 text-xs text-emerald-700 dark:text-emerald-300">
                  <CheckCircle2 className="size-3.5" />
                  Found
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                  <XCircle className="size-3.5" />
                  Missing
                </span>
              )}
            </div>
          ))}
        </div>
        {status.missing && status.missing.length > 0 ? (
          <p className="text-sm text-muted-foreground">
            Missing:{" "}
            <span className="font-mono text-foreground">
              {status.missing.join(", ")}
            </span>
          </p>
        ) : null}
      </section>

      {plan.packages?.length ? (
        <section className="space-y-4">
          <div className="flex items-center gap-2">
            <Box className="size-4 text-muted-foreground" />
            <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
              Packages to install
            </h2>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {plan.packages.map((pkg) => (
              <span
                key={pkg}
                className="rounded-md bg-muted px-2 py-0.5 font-mono text-xs"
              >
                {pkg}
              </span>
            ))}
          </div>
          {plan.optional_packages && plan.optional_packages.length > 0 ? (
            <div className="space-y-2">
              <p className="text-xs text-muted-foreground">Optional</p>
              <div className="flex flex-wrap gap-1.5">
                {plan.optional_packages.map((pkg) => (
                  <span
                    key={pkg}
                    className="rounded-md border border-dashed border-border px-2 py-0.5 font-mono text-xs text-muted-foreground"
                  >
                    {pkg}
                  </span>
                ))}
              </div>
            </div>
          ) : null}
        </section>
      ) : null}

      {plan.notes && plan.notes.length > 0 ? (
        <section className="space-y-3">
          <div className="flex items-center gap-2">
            <Info className="size-4 text-muted-foreground" />
            <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
              Notes
            </h2>
          </div>
          <ul className="space-y-2 text-sm text-muted-foreground">
            {plan.notes.map((note) => (
              <li key={note} className="leading-relaxed">
                {note}
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  )
}
