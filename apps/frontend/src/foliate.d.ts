// The parts of foliate-js (https://github.com/johnfactotum/foliate-js) novil uses.
declare module 'foliate-js/view.js' {
  export type TocItem = { label: string; href: string; subitems?: TocItem[] | null }

  export type Location = {
    cfi: string
    fraction: number
    tocItem?: { label: string; href: string }
  }

  export interface FoliateView extends HTMLElement {
    book: { toc?: TocItem[]; dir?: string }
    renderer: HTMLElement & { setStyles(css: string): void; next(): Promise<void>; prev(): Promise<void> }
    open(book: string | Blob): Promise<void>
    init(options: { lastLocation?: string; showTextStart?: boolean }): Promise<void>
    goTo(target: string): Promise<void>
    goToFraction(fraction: number): Promise<void>
    goLeft(): Promise<void>
    goRight(): Promise<void>
    close(): void
  }
}
