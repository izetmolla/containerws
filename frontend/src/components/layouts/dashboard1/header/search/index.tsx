"use client"

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from "react"
import { createPortal } from "react-dom"
import {
  CalendarIcon,
  CommandIcon,
  FileIcon,
  HashIcon,
  ImageIcon,
  MessageSquareIcon,
  SearchIcon,
  UsersIcon,
  VideoIcon,
} from "lucide-react"
import type { LucideIcon } from "lucide-react"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar"
import { cn } from "@/lib/utils"
import { foldSearchText, searchTextIncludes } from "../../lib/search-text"
import { useQuery } from "@tanstack/react-query"
import { useDebounce } from "../../hooks/use-debounce"
import { useNavigate } from "react-router"
import {
  isEmployeesGroup,
  isStudentsGroup,
  emptySearchFn,
  type DataResponse,
  type Employee,
  type Student,
  type SearchFn,
} from "./api"

export type { SearchFn }
type FlatNavItem = {
  title: string
  href: string
  icon?: LucideIcon
  group: string
}

type Person = {
  name: string
  initials: string
}

type DropdownPosition = {
  top: number
  left: number
  width: number
}

const searchFilters = [
  { id: "messages", label: "Messages", icon: MessageSquareIcon },
  { id: "files", label: "Files", icon: FileIcon },
  { id: "channels", label: "Channels", icon: HashIcon },
  { id: "chats", label: "Group chats", icon: UsersIcon },
  { id: "meetings", label: "Meetings", icon: VideoIcon },
  { id: "images", label: "Images", icon: ImageIcon },
] as const

const people: Person[] = [
  // { name: "Marcus Chen", initials: "MC" },
  // { name: "Sarah Williams", initials: "SW" },
  // { name: "James O'Brien", initials: "JO" },
  // { name: "Priya Sharma", initials: "PS" },
  // { name: "Elena Novak", initials: "EN" }
]

const DROPDOWN_GAP_PX = 8
const SEARCH_MAX_WIDTH_PX = 672 // max-w-2xl
const MOBILE_BREAKPOINT_PX = 1024

const searchInputClass = (open: boolean, inDropdown?: boolean) =>
  cn(
    "h-9 w-full rounded-lg border bg-background pr-12 pl-10 text-sm transition-[box-shadow,border-color,ring-color] duration-200",
    inDropdown
      ? "border-input/80 shadow-none"
      : open
        ? "border-border/80 shadow-sm ring-1 ring-ring/20"
        : "border-input/80 shadow-none hover:border-input"
  )

const searchDropdownClass =
  "bg-popover text-popover-foreground flex flex-col overflow-hidden rounded-xl border border-border/60 shadow-md ring-1 ring-foreground/5 dark:border-border/50 dark:ring-foreground/10"

const navItems: FlatNavItem[] = []
function flattenNavItems(): FlatNavItem[] {
  const result: FlatNavItem[] = []

  for (const item of navItems) {
    result.push(item)
  }

  return result
}

const allNavItems = flattenNavItems()

const searchWidthClass = "w-full max-w-2xl"

type SearchContextValue = {
  open: boolean
  query: string
  setQuery: (value: string) => void
  isMobile: boolean
  desktopAnchorRef: React.RefObject<HTMLDivElement | null>
  mobileTriggerRef: React.RefObject<HTMLDivElement | null>
  desktopInputRef: React.RefObject<HTMLInputElement | null>
  mobileInputRef: React.RefObject<HTMLInputElement | null>
  openSearch: () => void
  closeSearch: () => void
  activeFilter: (typeof searchFilters)[number]["id"] | null
  setActiveFilter: React.Dispatch<
    React.SetStateAction<(typeof searchFilters)[number]["id"] | null>
  >
  filteredPeople: Person[]
  filteredSuggestions: FlatNavItem[]
  searchGroups: DataResponse[]
  isSearchLoading: boolean
  hasSearchKeyword: boolean
  handleNavigate: (href: string) => void
}

const SearchContext = createContext<SearchContextValue | null>(null)

function useSearch() {
  const context = useContext(SearchContext)
  if (!context) {
    throw new Error("Search components must be used within SearchProvider")
  }
  return context
}

function useIsMobile() {
  return useSyncExternalStore(
    (callback) => {
      const media = window.matchMedia(
        `(max-width: ${MOBILE_BREAKPOINT_PX - 1}px)`
      )
      media.addEventListener("change", callback)
      return () => media.removeEventListener("change", callback)
    },
    () =>
      window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT_PX - 1}px)`).matches,
    () => false
  )
}

function useIsClient() {
  return useSyncExternalStore(
    () => () => {},
    () => true,
    () => false
  )
}

export function SearchProvider({
  children,
  searchFn = emptySearchFn,
}: {
  children: ReactNode
  searchFn?: SearchFn
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const [activeFilter, setActiveFilter] = useState<
    (typeof searchFilters)[number]["id"] | null
  >(null)
  const [position, setPosition] = useState<DropdownPosition | null>(null)
  const mounted = useIsClient()
  const isMobile = useIsMobile()
  const desktopAnchorRef = useRef<HTMLDivElement>(null)
  const mobileTriggerRef = useRef<HTMLDivElement>(null)
  const desktopInputRef = useRef<HTMLInputElement>(null)
  const mobileInputRef = useRef<HTMLInputElement>(null)
  const navigate = useNavigate()

  const focusInput = useCallback(() => {
    const input = isMobile ? mobileInputRef.current : desktopInputRef.current
    input?.focus()
  }, [isMobile])

  const updatePosition = useCallback(() => {
    const isMobileView = window.innerWidth < MOBILE_BREAKPOINT_PX
    const anchor = isMobileView
      ? mobileTriggerRef.current
      : desktopAnchorRef.current
    if (!anchor) return

    const rect = anchor.getBoundingClientRect()

    if (isMobileView) {
      const width = Math.min(window.innerWidth - 32, SEARCH_MAX_WIDTH_PX)
      setPosition({
        top: rect.bottom + DROPDOWN_GAP_PX,
        left: 16,
        width,
      })
      return
    }

    setPosition({
      top: rect.bottom + DROPDOWN_GAP_PX,
      left: rect.left,
      width: rect.width,
    })
  }, [])

  const openSearch = useCallback(() => {
    setOpen(true)
    requestAnimationFrame(() => {
      updatePosition()
      focusInput()
    })
  }, [updatePosition, focusInput])

  const closeSearch = useCallback(() => {
    setOpen(false)
  }, [])

  useEffect(() => {
    if (!open || !isMobile) return
    requestAnimationFrame(() => mobileInputRef.current?.focus())
  }, [open, isMobile])

  useEffect(() => {
    if (!open) return

    updatePosition()

    const onScrollOrResize = () => updatePosition()
    window.addEventListener("resize", onScrollOrResize)
    window.addEventListener("scroll", onScrollOrResize, true)

    return () => {
      window.removeEventListener("resize", onScrollOrResize)
      window.removeEventListener("scroll", onScrollOrResize, true)
    }
  }, [open, updatePosition])

  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        openSearch()
      }
      if (e.key === "Escape" && open) {
        e.preventDefault()
        closeSearch()
      }
    }
    document.addEventListener("keydown", down)
    return () => document.removeEventListener("keydown", down)
  }, [open, openSearch, closeSearch])

  const normalizedQuery = foldSearchText(query)
  const debouncedKeyword = useDebounce(query, 300).trim()

  const hasSearchKeyword = debouncedKeyword.length > 0

  const { data: searchResult, isFetching: isSearchLoading } = useQuery({
    queryKey: ["general", "modules", debouncedKeyword],
    queryFn: () => searchFn({ keyword: debouncedKeyword }),
    enabled: open && hasSearchKeyword,
    staleTime: 0,
    gcTime: 0,
  })
  const searchGroups = searchResult?.data ?? []

  const filteredPeople = useMemo(() => {
    const term = normalizedQuery.trim()
    if (!term) return people
    return people.filter((person) => searchTextIncludes(person.name, term))
  }, [normalizedQuery])

  const filteredSuggestions = useMemo(() => {
    let items = allNavItems
    const term = normalizedQuery.trim()

    if (term) {
      items = items.filter(
        (item) =>
          searchTextIncludes(item.title, term) ||
          searchTextIncludes(item.group, term)
      )
    }

    if (activeFilter === "files") {
      items = items.filter((item) => /file|folder|document/i.test(item.title))
    } else if (activeFilter === "messages" || activeFilter === "chats") {
      items = items.filter((item) => /chat|mail|message/i.test(item.title))
    } else if (activeFilter === "meetings") {
      items = items.filter((item) => /calendar|meeting|event/i.test(item.title))
    }

    return items.slice(0, 8)
  }, [normalizedQuery, activeFilter])

  const handleNavigate = useCallback(
    (href: string) => {
      closeSearch()
      setQuery("")
      navigate(href)
    },
    [closeSearch, setQuery, navigate]
  )

  const contextValue: SearchContextValue = {
    open,
    query,
    setQuery,
    isMobile,
    desktopAnchorRef,
    mobileTriggerRef,
    desktopInputRef,
    mobileInputRef,
    openSearch,
    closeSearch,
    activeFilter,
    setActiveFilter,
    filteredPeople,
    filteredSuggestions,
    searchGroups,
    isSearchLoading,
    hasSearchKeyword,
    handleNavigate,
  }

  return (
    <SearchContext.Provider value={contextValue}>
      {children}
      <SearchPortal
        mounted={mounted}
        position={position}
        open={open}
        closeSearch={closeSearch}
      />
    </SearchContext.Provider>
  )
}

function SearchInputField({
  inputRef,
  inDropdown = false,
}: {
  inputRef: React.RefObject<HTMLInputElement | null>
  inDropdown?: boolean
}) {
  const { open, query, setQuery, openSearch } = useSearch()

  return (
    <>
      <SearchIcon className="pointer-events-none absolute top-1/2 start-3 z-10 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        ref={inputRef}
        value={query}
        onChange={(e) => {
          setQuery(e.target.value)
          if (!open) openSearch()
        }}
        onClick={(e) => {
          e.stopPropagation()
          if (!open) openSearch()
        }}
        className={searchInputClass(open, inDropdown)}
        placeholder="Look for people, messages, files and more"
        type="search"
        autoComplete="off"
      />
      {!inDropdown && (
        <div className="pointer-events-none absolute top-1/2 end-2 hidden -translate-y-1/2 items-center gap-0.5 rounded-md border border-border/50 bg-muted px-1.5 py-0.5 font-mono text-xs font-medium text-muted-foreground sm:flex">
          <CommandIcon className="size-3" />
          <span>k</span>
        </div>
      )}
    </>
  )
}

function studentSearchSubtitle(student: Student) {
  return [
    student.document_id,
    student.program,
    student.email,
    student.faculty,
    student.study_level,
  ]
    .filter(Boolean)
    .join(" · ")
}

function initialsFromName(name: string) {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return "?"
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return `${parts[0][0] ?? ""}${parts[parts.length - 1][0] ?? ""}`.toUpperCase()
}

function SearchResultRow({
  name,
  subtitle,
  url,
  image,
}: {
  name: string
  subtitle: string
  url: string
  image?: string
}) {
  const navigate = useNavigate()
  const { closeSearch, setQuery } = useSearch()

  const handleNavigate = useCallback(() => {
    closeSearch()
    setQuery("")
    navigate(url)
  }, [closeSearch, setQuery, navigate, url])

  return (
    <li>
      <button
        type="button"
        className="flex w-full items-start gap-3 rounded-md px-2 py-2 text-start transition-colors hover:bg-muted"
        onMouseDown={(e) => e.preventDefault()}
        onClick={handleNavigate}
      >
        <Avatar className="mt-0.5 size-8 shrink-0">
          {image ? <AvatarImage src={image} alt={name} /> : null}
          <AvatarFallback className="text-xs">
            {initialsFromName(name)}
          </AvatarFallback>
        </Avatar>
        <span className="min-w-0 flex-1">
          <span className="line-clamp-1 text-sm font-medium">{name}</span>
          <span className="line-clamp-1 text-xs text-muted-foreground">
            {subtitle}
          </span>
        </span>
      </button>
    </li>
  )
}

function SearchDataGroups() {
  const { searchGroups, isSearchLoading, hasSearchKeyword } = useSearch()

  if (!hasSearchKeyword) return null

  if (isSearchLoading) {
    return (
      <section className="mb-3">
        <p className="px-2 py-4 text-center text-sm text-muted-foreground">
          Searching…
        </p>
      </section>
    )
  }

  const nonEmptyGroups = searchGroups.filter(
    (group) => Array.isArray(group.data) && group.data.length > 0
  )

  if (nonEmptyGroups.length === 0) {
    return (
      <section className="mb-3">
        <p className="px-2 py-4 text-center text-sm text-muted-foreground">
          No results found.
        </p>
      </section>
    )
  }

  return (
    <>
      {nonEmptyGroups.map((group) => (
        <section key={group.id} className="mb-3">
          <p className="mb-1 px-1 text-xs font-medium text-muted-foreground">
            {group.title}
          </p>
          <ul className="space-y-0.5">
            {isStudentsGroup(group) &&
              group.data.map((student: Student) => (
                <SearchResultRow
                  key={student.id}
                  name={`${student.full_name} (${student.reg_year}${student.reg_year ? `-${Number(student.reg_year) + 1}` : ""})`}
                  subtitle={studentSearchSubtitle(student)}
                  url={student?.url ?? "#"}
                />
              ))}
            {isEmployeesGroup(group) &&
              group.data.map((employee: Employee) => (
                <SearchResultRow
                  key={employee.id}
                  name={employee.full_name}
                  subtitle={employee.email ?? ""}
                  image={employee.image}
                  url={employee.url}
                />
              ))}
          </ul>
        </section>
      ))}
    </>
  )
}

function SearchDropdownContent() {
  const {
    isMobile,
    mobileInputRef,
    activeFilter,
    setActiveFilter,
    filteredPeople,
    filteredSuggestions,
    hasSearchKeyword,
    setQuery,
    handleNavigate,
  } = useSearch()

  return (
    <div className={searchDropdownClass}>
      {isMobile && (
        <div className="border-b border-border/50 p-2">
          <div className="relative">
            <SearchInputField inputRef={mobileInputRef} inDropdown />
          </div>
        </div>
      )}

      <div className="border-b border-border/50 p-2">
        <div className="flex gap-1.5 overflow-x-auto pb-0.5">
          {searchFilters.map((filter) => {
            const Icon = filter.icon
            const isActive = activeFilter === filter.id

            return (
              <Button
                key={filter.id}
                type="button"
                size="sm"
                variant={isActive ? "secondary" : "ghost"}
                className="h-8 shrink-0 rounded-full px-3 text-xs"
                onMouseDown={(e) => e.preventDefault()}
                onClick={() =>
                  setActiveFilter((current) =>
                    current === filter.id ? null : filter.id
                  )
                }
              >
                <Icon className="me-1.5 size-3.5" />
                {filter.label}
              </Button>
            )
          })}
        </div>
      </div>

      <div className="max-h-[50vh] overflow-y-auto overscroll-contain">
        <div className="p-2">
          <SearchDataGroups />

          {!hasSearchKeyword && filteredPeople.length > 0 && (
            <section className="mb-3">
              <p className="mb-2 px-1 text-xs font-medium text-muted-foreground">
                People
              </p>
              <div className="flex gap-3 overflow-x-auto px-1 pb-1">
                {filteredPeople.map((person) => (
                  <button
                    key={person.name}
                    type="button"
                    className="flex w-16 shrink-0 flex-col items-center gap-1.5 rounded-md p-1 text-center transition-colors hover:bg-muted"
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => setQuery(person.name)}
                  >
                    <Avatar className="size-10">
                      <AvatarFallback className="text-xs">
                        {person.initials}
                      </AvatarFallback>
                    </Avatar>
                    <span className="line-clamp-2 text-[11px] leading-tight">
                      {person.name}
                    </span>
                  </button>
                ))}
              </div>
            </section>
          )}

          {!hasSearchKeyword && (
            <section>
              <p className="mb-1 px-1 text-xs font-medium text-muted-foreground">
                Suggestions
              </p>
              {filteredSuggestions.length === 0 ? (
                <p className="px-2 py-6 text-center text-sm text-muted-foreground">
                  No results found.
                </p>
              ) : (
                <ul className="space-y-0.5">
                  {filteredSuggestions.map((item) => {
                    const Icon = item.icon ?? CalendarIcon

                    return (
                      <li key={`${item.href}-${item.title}`}>
                        <button
                          type="button"
                          className="flex w-full items-start gap-3 rounded-md px-2 py-2 text-start transition-colors hover:bg-muted"
                          onMouseDown={(e) => e.preventDefault()}
                          onClick={() => handleNavigate(item.href)}
                        >
                          <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                            <Icon className="size-4" />
                          </span>
                          <span className="min-w-0 flex-1">
                            <span className="line-clamp-1 text-sm font-medium">
                              {item.title}
                            </span>
                            <span className="line-clamp-1 text-xs text-muted-foreground">
                              {item.group}
                            </span>
                          </span>
                        </button>
                      </li>
                    )
                  })}
                </ul>
              )}
            </section>
          )}
        </div>
      </div>
    </div>
  )
}

function SearchPortal({
  mounted,
  position,
  open,
  closeSearch,
}: {
  mounted: boolean
  position: DropdownPosition | null
  open: boolean
  closeSearch: () => void
}) {
  if (!mounted || !open || !position) return null

  return createPortal(
    <>
      <button
        type="button"
        aria-label="Close search"
        className="fixed inset-0 z-40 animate-in bg-background/50 backdrop-blur-[2px] transition-opacity duration-200 fade-in-0"
        onClick={closeSearch}
      />
      <div
        className="fixed z-50 animate-in duration-200 fade-in-0 slide-in-from-top-1"
        style={{
          top: position.top,
          left: position.left,
          width: position.width,
        }}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <SearchDropdownContent />
      </div>
    </>,
    document.body
  )
}

export function SearchDesktop() {
  const { open, desktopAnchorRef, desktopInputRef, openSearch } = useSearch()

  return (
    <div
      ref={desktopAnchorRef}
      className={cn("relative hidden w-full lg:block", searchWidthClass)}
    >
      <div
        className={cn("relative w-full", open && "rounded-lg bg-background")}
        onClick={(e) => {
          if (e.target === e.currentTarget) openSearch()
        }}
        onKeyDown={(e) => {
          if (e.target !== e.currentTarget) return
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault()
            openSearch()
          }
        }}
        role="button"
        tabIndex={0}
      >
        <SearchInputField inputRef={desktopInputRef} />
      </div>
    </div>
  )
}

export function SearchMobileTrigger() {
  const { openSearch, mobileTriggerRef } = useSearch()

  return (
    <div ref={mobileTriggerRef} className="shrink-0 lg:hidden">
      <Button
        size="icon"
        variant="ghost"
        aria-label="Open search"
        onClick={openSearch}
      >
        <SearchIcon />
      </Button>
    </div>
  )
}

export default function Search() {
  return (
    <SearchProvider>
      <SearchDesktop />
      <SearchMobileTrigger />
    </SearchProvider>
  )
}
