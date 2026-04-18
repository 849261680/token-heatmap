import Foundation

struct TodayReportResponse: Decodable {
    let rows: [TodayReportRow]
}

struct TodayReportRow: Decodable {
    let day: String
    let provider: String
    let totalTokens: Int

    var providerDisplayName: String {
        switch provider {
        case "codex":   return "Codex"
        case "claude":  return "Claude Code"
        case "opencode":return "OpenCode"
        default:        return provider
        }
    }
}

struct UsageReport: Decodable {
    struct Row: Decodable {
        let day: String
        let totalTokens: Int
        let providers: [String: Int]

        enum CodingKeys: String, CodingKey {
            case day
            case totalTokens = "total_tokens"
            case providers
        }
    }
    let rows: [Row]
}

enum TokenHeatCLIError: LocalizedError {
    case missingCLI(String)
    case commandFailed(String)
    case invalidResponse

    var errorDescription: String? {
        switch self {
        case .missingCLI(let path):   return "找不到 tokenheat CLI：\(path)"
        case .commandFailed(let msg): return msg
        case .invalidResponse:        return "无法解析 tokenheat 的输出结果"
        }
    }
}

struct TokenHeatCLI {
    private let cliPath: String
    private let repoDir: String
    private let profileRepoDir: String?
    let profileURLString: String?
    let projectURLString: String?

    init(bundle: Bundle = .main) {
        let resourcesPath = bundle.resourceURL?.appendingPathComponent("tokenheat").path
        if let resourcesPath, FileManager.default.isExecutableFile(atPath: resourcesPath) {
            self.cliPath = resourcesPath
        } else {
            self.cliPath = bundle.object(forInfoDictionaryKey: "TokenHeatCLIPath") as? String
                ?? "/usr/local/bin/tokenheat"
        }
        self.repoDir = bundle.object(forInfoDictionaryKey: "TokenHeatRepoDir") as? String
            ?? FileManager.default.currentDirectoryPath
        self.profileRepoDir = bundle.object(forInfoDictionaryKey: "TokenHeatProfileRepoDir") as? String
        self.profileURLString  = bundle.object(forInfoDictionaryKey: "TokenHeatProfileURL") as? String
        self.projectURLString  = bundle.object(forInfoDictionaryKey: "TokenHeatProjectURL") as? String
    }

    func collect() async throws {
        _ = try await run(arguments: ["collect"])
    }

    func todayReport() async throws -> [TodayReportRow] {
        let output = try await run(arguments: ["report", "today", "--json"])
        let data = Data(output.utf8)
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        guard let response = try? decoder.decode(TodayReportResponse.self, from: data) else {
            throw TokenHeatCLIError.invalidResponse
        }
        return response.rows
    }

    func usageReport() async throws -> UsageReport {
        let url = URL(fileURLWithPath: repoDir)
            .appendingPathComponent("docs")
            .appendingPathComponent("usage.json")
        let data = try Data(contentsOf: url)
        return try JSONDecoder().decode(UsageReport.self, from: data)
    }

    func runDaily() async throws {
        var args = ["run", "daily", "--repo-dir", repoDir]
        if let profileRepoDir { args += ["--profile-repo-dir", profileRepoDir] }
        _ = try await run(arguments: args)
    }

    func installSchedule() async throws {
        var args = ["schedule", "install", "--repo-dir", repoDir, "--binary", cliPath]
        if let profileRepoDir { args += ["--profile-repo-dir", profileRepoDir] }
        _ = try await run(arguments: args)
    }

    func removeSchedule() async throws {
        _ = try await run(arguments: ["schedule", "remove"])
    }

    func scheduleInstalled() async throws -> Bool {
        let output = try await run(arguments: ["schedule", "status"])
        return output.contains("loaded: true")
    }

    private func run(arguments: [String]) async throws -> String {
        guard FileManager.default.isExecutableFile(atPath: cliPath) else {
            throw TokenHeatCLIError.missingCLI(cliPath)
        }
        return try await withCheckedThrowingContinuation { continuation in
            let process = Process()
            process.executableURL = URL(fileURLWithPath: cliPath)
            process.arguments = arguments
            process.currentDirectoryURL = URL(fileURLWithPath: repoDir)

            let stdout = Pipe()
            let stderr = Pipe()
            process.standardOutput = stdout
            process.standardError = stderr

            process.terminationHandler = { process in
                let out = String(decoding: stdout.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
                let err = String(decoding: stderr.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
                if process.terminationStatus == 0 {
                    continuation.resume(returning: out)
                } else {
                    continuation.resume(throwing: TokenHeatCLIError.commandFailed(
                        (err.isEmpty ? out : err).trimmingCharacters(in: .whitespacesAndNewlines)
                    ))
                }
            }
            do { try process.run() } catch { continuation.resume(throwing: error) }
        }
    }
}
