// Reads the QR code in a PNG with CoreImage and prints its text.
// Usage: swift coreimage.swift image.png
import CoreImage
import Foundation

guard CommandLine.arguments.count == 2,
      let image = CIImage(contentsOf: URL(fileURLWithPath: CommandLine.arguments[1])),
      let detector = CIDetector(ofType: CIDetectorTypeQRCode, context: nil,
                                options: [CIDetectorAccuracy: CIDetectorAccuracyHigh]) else {
    FileHandle.standardError.write("cannot open image or detector\n".data(using: .utf8)!)
    exit(2)
}
let features = detector.features(in: image).compactMap { $0 as? CIQRCodeFeature }
guard let text = features.first?.messageString else {
    FileHandle.standardError.write("no QR code found\n".data(using: .utf8)!)
    exit(1)
}
print(text)
