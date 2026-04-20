import * as React from "react"
import { cn } from "../../utils/cn"

export interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  error?: boolean;
}

const Select = React.forwardRef<HTMLSelectElement, SelectProps>(
  ({ className, error, ...props }, ref) => {
    return (
      <select
        className={cn(
          "flex h-10 w-full items-center justify-between rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-300 shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 disabled:cursor-not-allowed disabled:opacity-50 hover:bg-slate-800 transition-colors cursor-pointer",
          error 
            ? "border-red-500/50 focus:ring-red-500/50" 
            : "hover:border-slate-600",
          className
        )}
        ref={ref}
        {...props}
      />
    )
  }
)
Select.displayName = "Select"

export { Select }
