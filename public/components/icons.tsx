import type { SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement>;

/** 渲染统一的线性图标画布。 */
function Icon({ children, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" {...props}>
      {children}
    </svg>
  );
}

/** 渲染立方体软件包图标。 */
export function PackageIcon(props: IconProps) {
  return <Icon {...props}><path d="m12 3 8 4.5v9L12 21l-8-4.5v-9L12 3Z"/><path d="m4.3 7.7 7.7 4.4 7.7-4.4M12 12v9M8 5.2l8 4.6"/></Icon>;
}

/** 渲染闪电自动化图标。 */
export function SparkIcon(props: IconProps) {
  return <Icon {...props}><path d="m13 2-8 12h7l-1 8 8-12h-7l1-8Z"/></Icon>;
}

/** 渲染加号图标。 */
export function PlusIcon(props: IconProps) {
  return <Icon {...props}><path d="M12 5v14M5 12h14"/></Icon>;
}

/** 渲染链接图标。 */
export function LinkIcon(props: IconProps) {
  return <Icon {...props}><path d="M10 13a5 5 0 0 0 7.1.1l2-2a5 5 0 0 0-7.1-7.1l-1.1 1.1M14 11a5 5 0 0 0-7.1-.1l-2 2A5 5 0 0 0 12 20l1.1-1.1"/></Icon>;
}

/** 渲染时钟图标。 */
export function ClockIcon(props: IconProps) {
  return <Icon {...props}><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></Icon>;
}

/** 渲染终端窗口图标。 */
export function TerminalIcon(props: IconProps) {
  return <Icon {...props}><rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7 9 3 3-3 3M13 15h4"/></Icon>;
}

/** 渲染圆形对勾图标。 */
export function CheckIcon(props: IconProps) {
  return <Icon {...props}><circle cx="12" cy="12" r="9"/><path d="m8 12 2.5 2.5L16 9"/></Icon>;
}

/** 渲染下载图标。 */
export function DownloadIcon(props: IconProps) {
  return <Icon {...props}><path d="M12 3v12m-4-4 4 4 4-4M5 20h14"/></Icon>;
}

/** 渲染关闭图标。 */
export function CloseIcon(props: IconProps) {
  return <Icon {...props}><path d="m6 6 12 12M18 6 6 18"/></Icon>;
}

/** 渲染上传发布图标。 */
export function UploadIcon(props: IconProps) {
  return <Icon {...props}><path d="M12 16V4m-4 4 4-4 4 4M5 14v5h14v-5"/></Icon>;
}

/** 渲染外部链接图标。 */
export function ExternalIcon(props: IconProps) {
  return <Icon {...props}><path d="M14 4h6v6M20 4l-9 9M18 13v6H5V6h6"/></Icon>;
}

/** 渲染文档文件图标。 */
export function FileIcon(props: IconProps) {
  return <Icon {...props}><path d="M6 3h8l4 4v14H6V3Z"/><path d="M14 3v5h4M9 13h6M9 17h5"/></Icon>;
}

/** 渲染搜索图标。 */
export function SearchIcon(props: IconProps) {
  return <Icon {...props}><circle cx="11" cy="11" r="7"/><path d="m16 16 4 4"/></Icon>;
}

/** 渲染重试刷新图标。 */
export function RefreshIcon(props: IconProps) {
  return <Icon {...props}><path d="M20 7v5h-5M4 17v-5h5"/><path d="M6.1 8a7 7 0 0 1 11.7-2L20 8M4 16l2.2 2A7 7 0 0 0 18 16"/></Icon>;
}

/** 渲染删除图标。 */
export function TrashIcon(props: IconProps) {
  return <Icon {...props}><path d="M4 7h16M9 3h6l1 4H8l1-4ZM7 7l1 14h8l1-14M10 11v6M14 11v6"/></Icon>;
}

/** 渲染退出登录图标。 */
export function LogoutIcon(props: IconProps) {
  return <Icon {...props}><path d="M10 4H4v16h6M14 8l4 4-4 4M8 12h10"/></Icon>;
}

/** 渲染暂停自动滚动图标。 */
export function PauseIcon(props: IconProps) {
  return <Icon {...props}><path d="M8 5v14M16 5v14"/></Icon>;
}

/** 渲染继续播放图标。 */
export function PlayIcon(props: IconProps) {
  return <Icon {...props}><path d="m8 5 11 7-11 7V5Z"/></Icon>;
}
