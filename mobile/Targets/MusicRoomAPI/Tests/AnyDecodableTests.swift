import XCTest

@testable import MusicRoomAPI

final class AnyDecodableTests: XCTestCase {
    func testDecodeNull() throws {
        let json = "null".data(using: .utf8)!
        let decoded = try JSONDecoder().decode(AnyDecodable.self, from: json)
        XCTAssertTrue(decoded.value is NSNull)
    }

    func testDecodeInt() throws {
        let json = "42".data(using: .utf8)!
        let decoded = try JSONDecoder().decode(AnyDecodable.self, from: json)
        XCTAssertEqual(decoded.value as? Int, 42)
    }

    func testDecodeString() throws {
        let json = "\"hello\"".data(using: .utf8)!
        let decoded = try JSONDecoder().decode(AnyDecodable.self, from: json)
        XCTAssertEqual(decoded.value as? String, "hello")
    }

    func testDecodeBool() throws {
        let json = "true".data(using: .utf8)!
        let decoded = try JSONDecoder().decode(AnyDecodable.self, from: json)
        XCTAssertEqual(decoded.value as? Bool, true)
    }

    func testDecodeArray() throws {
        let json = "[1, \"two\", true, null]".data(using: .utf8)!
        let decoded = try JSONDecoder().decode(AnyDecodable.self, from: json)
        guard let array = decoded.value as? [Any] else {
            XCTFail("Expected array")
            return
        }
        XCTAssertEqual(array.count, 4)
        XCTAssertEqual(array[0] as? Int, 1)
        XCTAssertEqual(array[1] as? String, "two")
        XCTAssertEqual(array[2] as? Bool, true)
        XCTAssertTrue(array[3] is NSNull)
    }

    func testDecodeDictionary() throws {
        let json = "{\"key\": \"value\", \"nested\": {\"inner\": null}}".data(using: .utf8)!
        let decoded = try JSONDecoder().decode(AnyDecodable.self, from: json)
        guard let dict = decoded.value as? [String: Any] else {
            XCTFail("Expected dictionary")
            return
        }
        XCTAssertEqual(dict["key"] as? String, "value")
        guard let nested = dict["nested"] as? [String: Any] else {
            XCTFail("Expected nested dictionary")
            return
        }
        XCTAssertTrue(nested["inner"] is NSNull)
    }

    func testEncodeNull() throws {
        let anyDecodable = AnyDecodable(NSNull())
        let encoded = try JSONEncoder().encode(anyDecodable)
        XCTAssertEqual(String(data: encoded, encoding: .utf8), "null")
    }

    func testEquality() {
        XCTAssertEqual(AnyDecodable(NSNull()), AnyDecodable(NSNull()))
        XCTAssertNotEqual(AnyDecodable(NSNull()), AnyDecodable("nil"))
        XCTAssertEqual(AnyDecodable(42), AnyDecodable(42))
        XCTAssertEqual(AnyDecodable("test"), AnyDecodable("test"))
    }
}
