export type TaskStatus =
  | "queued"
  | "discovering"
  | "generating"
  | "building"
  | "succeeded"
  | "failed"
  | "canceled"
  | "publishing"
  | "published";

export interface LogEntry {
  time: string;
  level: string;
  message: string;
}

export interface Publication {
  repository: string;
  branch: string;
  commit: string;
  package_url: string;
  pushed_at: string;
}

export interface AgentTask {
  id: string;
  source_url: string;
  package_name: string;
  version: string;
  description: string;
  upstream_url: string;
  source_archive_url: string;
  status: TaskStatus;
  publish_requested: boolean;
  attempts: number;
  pkgbuild: string;
  srcinfo: string;
  build_log: string;
  error: string;
  logs: LogEntry[];
  publication?: Publication;
  created_at: string;
  updated_at: string;
  finished_at?: string;
}

export interface Health {
  status: string;
  docker: string;
  worker: {
    concurrency: number;
    running: number;
    queued: number;
  };
  time: string;
}

export interface AuthStatus {
  enabled: boolean;
  authenticated: boolean;
}
