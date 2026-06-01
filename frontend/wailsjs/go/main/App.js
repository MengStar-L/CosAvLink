// Generated Go binding stubs - replaced by wails build
// These stubs allow TypeScript compilation during development.

export function GetVideos(page: number): Promise<any> {
  return (window as any)['go']['main']['App']['GetVideos'](page);
}

export function GetMagnets(code: string, title: string): Promise<any> {
  return (window as any)['go']['main']['App']['GetMagnets'](code, title);
}
