import SwiftUI

struct MenuBarContentView: View {
    @EnvironmentObject private var viewModel: MenuBarViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 4) {
                    Text("Token Heatmap")
                        .font(.headline)
                    Text(viewModel.totalSummary)
                        .font(.title3.weight(.semibold))
                    Text(viewModel.lastUpdatedSummary)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                if viewModel.isRefreshing {
                    ProgressView()
                        .controlSize(.small)
                }
            }

            Divider()

            VStack(alignment: .leading, spacing: 8) {
                ForEach(viewModel.providerSummaries) { summary in
                    HStack {
                        Text(summary.name)
                            .font(.subheadline.weight(.medium))
                        Spacer()
                        Text(summary.tokensText)
                            .font(.subheadline.monospacedDigit())
                            .foregroundStyle(.secondary)
                    }
                }
                if viewModel.providerSummaries.isEmpty {
                    Text("No usage rows for today")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
            }

            Divider()

            VStack(alignment: .leading, spacing: 6) {
                Label(viewModel.scheduleStatusText, systemImage: viewModel.scheduleInstalled ? "clock.badge.checkmark" : "clock.badge.xmark")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
                if let error = viewModel.lastError {
                    Text(error)
                        .font(.footnote)
                        .foregroundStyle(.red)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }

            Divider()

            VStack(alignment: .leading, spacing: 8) {
                Button("Refresh") {
                    viewModel.refresh()
                }
                Button("Sync Now") {
                    viewModel.syncNow()
                }
                Button(viewModel.scheduleInstalled ? "Reinstall Daily Sync" : "Install Daily Sync") {
                    viewModel.installSchedule()
                }
                Button("Remove Daily Sync") {
                    viewModel.removeSchedule()
                }
                .disabled(!viewModel.scheduleInstalled)
            }

            Divider()

            VStack(alignment: .leading, spacing: 8) {
                Button("Open GitHub Profile") {
                    viewModel.openProfile()
                }
                Button("Open Project Repo") {
                    viewModel.openProjectRepo()
                }
                Button("Quit") {
                    viewModel.quit()
                }
            }
        }
        .padding(14)
        .task {
            viewModel.start()
        }
    }
}
