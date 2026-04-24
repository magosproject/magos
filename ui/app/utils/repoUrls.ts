export function revisionUrl(repoURL: string, revision: string): string | null {
  if (!repoURL || !revision) return null;

  const base = repoURL.replace(/\.git$/, "");
  if (base.includes("github.com") || base.includes("gitlab.com") || base.includes("gitlab."))
    return `${base}/tree/${revision}`;
  if (base.includes("bitbucket.org")) return `${base}/src/${revision}`;
  return null;
}

export function commitUrl(repoURL: string, sha: string): string | null {
  if (!repoURL || !sha) return null;

  const base = repoURL.replace(/\.git$/, "");
  if (base.includes("gitlab")) return `${base}/-/commit/${sha}`;
  if (base.includes("bitbucket")) return `${base}/commits/${sha}`;
  return `${base}/commit/${sha}`;
}

export function terraformReleaseUrl(version: string): string | null {
  if (!version) return null;
  return `https://releases.hashicorp.com/terraform/${version}`;
}
