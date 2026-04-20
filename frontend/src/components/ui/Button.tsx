import React from "react"
import { cn } from "../../utils/cn"

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "default" | "primary" | "destructive" | "ghost" | "outline"
  size?: "default" | "sm" | "lg" | "icon"
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "default", size = "default", ...props }, ref) => {
    return (
      <button
        ref={ref}
        className={cn(
          "inline-flex items-center justify-center rounded-lg font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-slate-400 focus:ring-offset-2 disabled:opacity-50 disabled:pointer-events-none",
          
          // Variants
          variant === "default" && "bg-slate-800 text-slate-100 hover:bg-slate-700",
          variant === "primary" && "bg-blue-600 text-white hover:bg-blue-500 focus:ring-blue-500 focus:ring-offset-slate-900",
          variant === "destructive" && "bg-red-500/10 text-red-500 hover:bg-red-500/20 border border-transparent hover:border-red-500/30",
          variant === "ghost" && "hover:bg-slate-800 text-slate-300 hover:text-white",
          variant === "outline" && "border border-slate-700 bg-transparent hover:bg-slate-800 text-slate-300",
          
          // Sizes
          size === "default" && "h-10 py-2 px-4 text-sm",
          size === "sm" && "h-8 px-3 text-xs rounded-md",
          size === "lg" && "h-12 px-8 text-base rounded-md",
          size === "icon" && "h-10 w-10",
          
          className
        )}
        {...props}
      />
    )
  }
)
Button.displayName = "Button"

export { Button }
