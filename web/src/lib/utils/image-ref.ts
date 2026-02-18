export interface ImageRefParts {
    original: string;
    repository: string;
    tag: string;
    digest: string;
    qualifier: string;
}

function shortDigest(digest: string): string {
    const value = String(digest || "").trim();
    if (!value) return "";
    return value.length > 18 ? `${value.slice(0, 18)}...` : value;
}

export function parseImageRef(ref: string): ImageRefParts {
    const original = String(ref || "").trim();
    if (!original) {
        return {
            original,
            repository: "<none>",
            tag: "latest",
            digest: "",
            qualifier: ":latest"
        };
    }

    const at = original.indexOf("@");
    const namePart = at > 0 ? original.slice(0, at) : original;
    const digest = at > 0 ? original.slice(at + 1).trim() : "";

    const lastSlash = namePart.lastIndexOf("/");
    const lastColon = namePart.lastIndexOf(":");

    let repository = namePart.trim();
    let tag = "";
    if (lastColon > lastSlash) {
        repository = namePart.slice(0, lastColon).trim();
        tag = namePart.slice(lastColon + 1).trim();
    }

    if (!repository) {
        repository = "<none>";
    }
    if (!tag && !digest) {
        tag = "latest";
    }

    const qualifier = tag ? `:${tag}` : shortDigest(digest);

    return {
        original,
        repository,
        tag,
        digest,
        qualifier
    };
}
