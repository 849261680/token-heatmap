import AppKit
import Foundation
import SwiftUI

@MainActor
final class MenuBarViewModel: ObservableObject {
    struct ProviderSummary: Identifiable {
        let id: String
        let name: String
        let totalTokens: Int

        var tokensText: String {
            NumberFormatter.tokenFormatter.string(from: NSNumber(value: totalTokens)) ?? "\(totalTokens)"
        }
    }

    @Published private(set) var providerSummaries: [ProviderSummary] = []
    @Published private(set) var totalTokens: Int = 0
    @Published private(set) var lastUpdated: Date?
    @Published private(set) var isRefreshing = false
    @Published private(set) var scheduleInstalled = false
    @Published private(set) var lastError: String?

    private let cli = TokenHeatCLI()
    private var didStart = false
    private var refreshTask: Task<Void, Never>?

    var menuTitle: String {
        if totalTokens == 0 {
            return "TH"
        }
        return "TH \(compactTokenString(totalTokens))"
    }

    var totalSummary: String {
        if totalTokens == 0 {
            return "No tokens today"
        }
        return "\(compactTokenString(totalTokens)) tokens today"
    }

    var lastUpdatedSummary: String {
        guard let lastUpdated else { return "Not refreshed yet" }
        return "Updated \(RelativeDateTimeFormatter().localizedString(for: lastUpdated, relativeTo: Date()))"
    }

    var scheduleStatusText: String {
        scheduleInstalled ? "Daily sync installed" : "Daily sync not installed"
    }

    func start() {
        guard !didStart else { return }
        didStart = true
        refresh()
        refreshTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(120))
                await self?.refresh()
            }
        }
    }

    func refresh() {
        guard !isRefreshing else { return }
        isRefreshing = true
        lastError = nil

        Task {
            do {
                async let report = cli.todayReport()
                async let scheduleInstalled = cli.scheduleInstalled()
                let (rows, schedule) = try await (report, scheduleInstalled)

                providerSummaries = rows
                    .map { row in
                        ProviderSummary(
                            id: row.provider,
                            name: row.providerDisplayName,
                            totalTokens: row.totalTokens
                        )
                    }
                    .sorted { $0.name < $1.name }
                totalTokens = rows.reduce(0) { $0 + $1.totalTokens }
                self.scheduleInstalled = schedule
                lastUpdated = Date()
            } catch {
                lastError = error.localizedDescription
            }
            isRefreshing = false
        }
    }

    func syncNow() {
        runAction {
            try await cli.runDaily()
        }
    }

    func installSchedule() {
        runAction {
            try await cli.installSchedule()
        }
    }

    func removeSchedule() {
        runAction {
            try await cli.removeSchedule()
        }
    }

    func openProfile() {
        open(urlString: cli.profileURLString)
    }

    func openProjectRepo() {
        open(urlString: cli.projectURLString)
    }

    func quit() {
        NSApplication.shared.terminate(nil)
    }

    private func runAction(_ operation: @escaping () async throws -> Void) {
        guard !isRefreshing else { return }
        isRefreshing = true
        lastError = nil
        Task {
            do {
                try await operation()
                isRefreshing = false
                refresh()
            } catch {
                lastError = error.localizedDescription
                isRefreshing = false
            }
        }
    }

    private func open(urlString: String?) {
        guard let urlString, let url = URL(string: urlString) else { return }
        NSWorkspace.shared.open(url)
    }

    private func compactTokenString(_ value: Int) -> String {
        let number = Double(value)
        if number >= 1_000_000_000 {
            return String(format: "%.1fB", number / 1_000_000_000)
        }
        if number >= 1_000_000 {
            return String(format: "%.1fM", number / 1_000_000)
        }
        if number >= 1_000 {
            return String(format: "%.1fK", number / 1_000)
        }
        return "\(value)"
    }
}

private extension NumberFormatter {
    static let tokenFormatter: NumberFormatter = {
        let formatter = NumberFormatter()
        formatter.numberStyle = .decimal
        return formatter
    }()
}
