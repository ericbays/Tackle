import * as React from "react"
import { cn } from "../../utils/cn"

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "secondary" | "destructive" | "outline" | "success" | "warning"
}

function Badge({ className, variant = "default", ...props }: BadgeProps) {
  return (
    <div
      className={cn(
        "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-slate-400 focus:ring-offset-2",
        
        variant === "default" && "border-transparent bg-blue-600 text-white hover:bg-blue-500",
        variant === "secondary" && "border-transparent bg-slate-800 text-slate-100 hover:bg-slate-700",
        variant === "destructive" && "border-transparent bg-red-500/10 text-red-500 border border-red-500/20",
        variant === "success" && "border-transparent bg-emerald-500/10 text-emerald-400 border border-emerald-500/20",
        variant === "warning" && "border-transparent bg-orange-500/10 text-orange-500 border border-orange-500/20",
        variant === "outline" && "text-slate-300 border-slate-700",
        
        className
      )}
      {...props}
    />
  )
}

export { Badge }
