import AppKit
import Foundation
import SwiftUI

@MainActor
final class MenuBarViewModel: ObservableObject {

    struct HeatmapDay {
        let date: String
        let tokens: Int
        var level: Int {
            switch tokens {
            case 0:               return 0
            case 1..<1_000_000:   return 1
            case 1_000_000..<10_000_000:  return 2
            case 10_000_000..<100_000_000: return 3
            default:              return 4
            }
        }
    }

    struct ProviderSummary: Identifiable {
        let id: String
        let name: String
        let totalTokens: Int

        var tokensText: String { compactTokenString(totalTokens) }
    }

    @Published private(set) var todayTokens: Int = 0
    @Published private(set) var weeklyTokens: Int = 0
    @Published private(set) var primaryProvider: String = "—"
    @Published private(set) var heatmapDays: [HeatmapDay] = []
    @Published private(set) var lastUpdated: Date?
    @Published private(set) var isRefreshing = false
    @Published private(set) var scheduleInstalled = false
    @Published private(set) var lastError: String?

    private let cli = TokenHeatCLI()
    private var didStart = false
    private var refreshTask: Task<Void, Never>?

    var menuTitle: String {
        todayTokens == 0 ? "热图" : "热图 \(compactTokenString(todayTokens))"
    }

    var todaySummary: String {
        todayTokens == 0 ? "暂无数据" : compactTokenString(todayTokens)
    }

    var weeklySummary: String {
        weeklyTokens == 0 ? "暂无数据" : compactTokenString(weeklyTokens)
    }

    var lastUpdatedSummary: String {
        guard let lastUpdated else { return "尚未刷新" }
        let f = RelativeDateTimeFormatter()
        f.locale = Locale(identifier: "zh_CN")
        return f.localizedString(for: lastUpdated, relativeTo: Date())
    }

    func start() {
        guard !didStart else { return }
        didStart = true
        refresh()
        refreshTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(120))
                self?.refresh()
            }
        }
    }

    func refresh() {
        guard !isRefreshing else { return }
        isRefreshing = true
        lastError = nil

        Task {
            do {
                try await cli.collect()
                async let todayRows   = cli.todayReport()
                async let usageReport = cli.usageReport()
                async let scheduled   = cli.scheduleInstalled()

                let (rows, usage, sched) = try await (todayRows, usageReport, scheduled)

                todayTokens = rows.reduce(0) { $0 + $1.totalTokens }

                // primary provider (highest today)
                primaryProvider = rows.max(by: { $0.totalTokens < $1.totalTokens })
                    .map { $0.providerDisplayName } ?? "—"

                // weekly total from usage.json (last 7 days)
                let today = Calendar.current.startOfDay(for: Date())
                let sevenDaysAgo = Calendar.current.date(byAdding: .day, value: -6, to: today)!
                let fmt = DateFormatter()
                fmt.dateFormat = "yyyy-MM-dd"
                weeklyTokens = usage.rows
                    .filter { row in
                        guard let d = fmt.date(from: row.day) else { return false }
                        return d >= sevenDaysAgo
                    }
                    .reduce(0) { $0 + $1.totalTokens }

                // heatmap: last 98 days (14 weeks × 7)
                let ninetyEightDaysAgo = Calendar.current.date(byAdding: .day, value: -97, to: today)!
                let usageMap = Dictionary(uniqueKeysWithValues: usage.rows.map { ($0.day, $0.totalTokens) })
                var days: [HeatmapDay] = []
                for offset in 0..<98 {
                    let date = Calendar.current.date(byAdding: .day, value: offset, to: ninetyEightDaysAgo)!
                    let key = fmt.string(from: date)
                    days.append(HeatmapDay(date: key, tokens: usageMap[key] ?? 0))
                }
                heatmapDays = days

                scheduleInstalled = sched
                lastUpdated = Date()
            } catch {
                lastError = error.localizedDescription
            }
            isRefreshing = false
        }
    }

    func syncNow() {
        runAction { try await self.cli.runDaily() }
    }

    func setScheduleEnabled(_ enabled: Bool) {
        guard enabled != scheduleInstalled, !isRefreshing else { return }
        let previous = scheduleInstalled
        scheduleInstalled = enabled
        isRefreshing = true
        lastError = nil
        Task {
            do {
                if enabled { try await cli.installSchedule() }
                else        { try await cli.removeSchedule() }
                isRefreshing = false
                refresh()
            } catch {
                scheduleInstalled = previous
                lastError = error.localizedDescription
                isRefreshing = false
            }
        }
    }

    func openHeatmap() {
        guard let s = cli.profileURLString, let url = URL(string: s) else { return }
        NSWorkspace.shared.open(url)
    }

    func quit() { NSApplication.shared.terminate(nil) }

    private func runAction(_ op: @escaping () async throws -> Void) {
        guard !isRefreshing else { return }
        isRefreshing = true
        lastError = nil
        Task {
            do {
                try await op()
                isRefreshing = false
                refresh()
            } catch {
                lastError = error.localizedDescription
                isRefreshing = false
            }
        }
    }
}

private func compactTokenString(_ value: Int) -> String {
    let n = Double(value)
    if n >= 1_000_000_000 { return String(format: "%.1fB", n / 1_000_000_000) }
    if n >= 1_000_000     { return String(format: "%.1fM", n / 1_000_000) }
    if n >= 1_000         { return String(format: "%.1fK", n / 1_000) }
    return "\(value)"
}
