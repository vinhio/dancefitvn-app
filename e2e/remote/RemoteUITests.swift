// Adapted from jplayer-tv/e2e/remote/AppRemoteUITests.swift at 781bee5.
//
// RemoteUITests presses the Siri Remote's buttons on the Apple TV
// simulator through XCUIRemote (XCTest synthesizes the real UIPresses: no
// Accessibility permission on the Mac, no osascript) while DanceFITVN,
// launched beforehand by scripts/tvos-remote.sh with its console in a log,
// runs. gDesk draws the whole interface itself, so XCTest cannot see the
// focus: the screenshots are the evidence, and each step prints a marker
// line "dfvremote: <unix time> <what>" the script keeps.
//
// The steps come from DFV_STEPS (TEST_RUNNER_DFV_STEPS on the host),
// separated by spaces (local/plans/P7.md section 8.1):
//   up down left right select menu playpause home
//                  press, then wait 600 ms; "<button>@<ms>" waits <ms>
//   hold:<ms>      hold Select <ms>, then wait 600 ms
//   wait:<ms>      wait
//   since:<ms>     wait until <ms> after the last press
//   shot:<name>    save the screen as <DFV_SHOT>-<name>.png
//   front          bring DanceFITVN to the front again (after home)
//   state:<name>   print the app's state (foreground, background ...)
//   mark:<text>    print a marker, for the script to cut the app's log
import XCTest

@MainActor
final class RemoteUITests: XCTestCase {
    /// DanceFITVN, already launched by the script: activate does not relaunch
    /// it. DFV_APP names another app (scripts/tvos-avf-test.sh device-e2e
    /// sends its engine test app home and back with "home" and "front").
    let app = XCUIApplication(bundleIdentifier: {
        let id = ProcessInfo.processInfo.environment["DFV_APP"] ?? ""
        return id.isEmpty ? "com.jivecode.dancefitvn" : id
    }())
    let remote = XCUIRemote.shared

    /// The pause after a press, for the focus animation (150 ms) to settle.
    let settle = 600

    static let buttons: [String: XCUIRemote.Button] = [
        "up": .up, "down": .down, "left": .left, "right": .right,
        "select": .select, "menu": .menu, "playpause": .playPause, "home": .home,
    ]

    func testRemoteSteps() throws {
        app.activate()
        XCTAssertTrue(app.wait(for: .runningForeground, timeout: 15), "DanceFITVN is not in the foreground")
        mark("start \(stateName(app.state))")

        let env = ProcessInfo.processInfo.environment["DFV_STEPS"] ?? ""
        let steps = (env.isEmpty ? "shot:launch" : env).split(separator: " ").map(String.init)
        var last = Date()
        for step in steps {
            let parts = step.split(separator: ":", maxSplits: 1).map(String.init)
            let name = parts[0], arg = parts.count > 1 ? parts[1] : ""
            switch name {
            case "shot":
                shoot(arg)
            case "wait":
                pause(ms: Int(arg) ?? 0)
            case "since":
                pause(ms: (Int(arg) ?? 0) - Int(Date().timeIntervalSince(last) * 1000))
            case "hold":
                mark("step hold-select \(arg)ms")
                remote.press(.select, forDuration: Double(Int(arg) ?? 0) / 1000)
                last = Date()
                pause(ms: settle)
            case "front":
                mark("step front")
                app.activate()
                _ = app.wait(for: .runningForeground, timeout: 15)
                last = Date()
                pause(ms: settle)
            case "state":
                mark("state \(arg) \(stateName(app.state))")
            case "mark":
                mark("mark \(arg)")
            default:
                let pair = step.split(separator: "@", maxSplits: 1).map(String.init)
                guard let button = Self.buttons[pair[0]] else {
                    XCTFail("unknown step \(step)")
                    return
                }
                mark("step \(step)")
                remote.press(button)
                last = Date()
                pause(ms: pair.count > 1 ? (Int(pair[1]) ?? settle) : settle)
            }
        }
        mark("state-at-end \(stateName(app.state))")
    }

    func pause(ms: Int) {
        if ms > 0 {
            Thread.sleep(forTimeInterval: Double(ms) / 1000)
        }
    }

    /// shoot saves the screen as a PNG named after DFV_SHOT (a path prefix
    /// the script passes as TEST_RUNNER_DFV_SHOT), if set. On an Apple TV
    /// the runner cannot write to the Mac: DFV_SHOT "attach:<prefix>"
    /// (scripts/tvos-remote-device.sh) keeps the shot as an attachment of
    /// the result bundle, named "<prefix>-<name>", which the script exports.
    func shoot(_ name: String) {
        guard let prefix = ProcessInfo.processInfo.environment["DFV_SHOT"], !prefix.isEmpty else { return }
        if prefix.hasPrefix("attach:") {
            let shot = XCTAttachment(screenshot: XCUIScreen.main.screenshot())
            shot.name = "\(prefix.dropFirst("attach:".count))-\(name)"
            shot.lifetime = .keepAlways
            add(shot)
            mark("shot attach \(shot.name ?? name)")
            return
        }
        let url = URL(fileURLWithPath: "\(prefix)-\(name).png")
        do {
            try XCUIScreen.main.screenshot().pngRepresentation.write(to: url)
            mark("shot \(url.path)")
        } catch {
            mark("shot-failed \(error)")
        }
    }

    func mark(_ what: String) {
        print("dfvremote: \(String(format: "%.3f", Date().timeIntervalSince1970)) \(what)")
    }

    func stateName(_ s: XCUIApplication.State) -> String {
        switch s {
        case .notRunning: return "notRunning"
        case .runningBackgroundSuspended: return "runningBackgroundSuspended"
        case .runningBackground: return "runningBackground"
        case .runningForeground: return "runningForeground"
        default: return "unknown"
        }
    }
}
