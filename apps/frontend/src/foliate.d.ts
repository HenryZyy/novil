// The parts of foliate-js (https://github.com/johnfactotum/foliate-js) novil uses.
declare module 'foliate-js/view.js' {
  export type TocItem = { label: string; href: string; subitems?: TocItem[] | null }

  export type Location = {
    // The range shown, from its first to its last character.
    cfi: string
    range?: Range
    fraction: number
    tocItem?: { label: string; href: string }
  }

  // value is a CFI; the rest is passed back in draw-annotation.
  export type ViewAnnotation = { value: string; color?: string }

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
    // Draws an annotation, through a draw-annotation event, where its
    // section is shown; deleteAnnotation removes it.
    addAnnotation(annotation: ViewAnnotation): Promise<unknown>
    deleteAnnotation(annotation: ViewAnnotation): Promise<unknown>
    getCFI(index: number, range: Range): string
  }

  // Events beyond relocate and load.
  export type DrawAnnotationDetail = {
    draw: (func: unknown, options: { color: string }) => void
    annotation: ViewAnnotation
  }
  export type ShowAnnotationDetail = { value: string; range: Range }
}

declare module 'foliate-js/overlayer.js' {
  export class Overlayer {
    static highlight: unknown
  }
}

declare module 'foliate-js/epubcfi.js' {
  // The start of a range CFI, or its end with toEnd.
  export function collapse(cfi: string, toEnd?: boolean): string
  // Negative, 0 or positive, like a sort comparator.
  export function compare(a: string, b: string): number
}
