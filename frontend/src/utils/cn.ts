import { clsx, type ClassValue } from "clsx"
import { extendTailwindMerge } from "tailwind-merge"

const customTwMerge = extendTailwindMerge({
  extend: {
    // Add any custom theme extensions here if needed
  }
})

export function cn(...inputs: ClassValue[]) {
  return customTwMerge(clsx(inputs))
}
