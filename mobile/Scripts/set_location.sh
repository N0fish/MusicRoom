#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Music Room Simulator Location Setter ===${NC}"

# Get list of booted simulators UUIDs and Names
# Format: "UUID SetName"
# We actally just want to parse `xcrun simctl list devices`
# Raw output example:
#    iPhone 14 (79946809-3221-4191-807B-57A1A7D17300) (Booted)

# Store in arrays
declare -a UUIDS
declare -a NAMES

IFS=$'\n'
lines=($(xcrun simctl list devices | grep "(Booted)"))
unset IFS

if [ ${#lines[@]} -eq 0 ]; then
    echo "No booted simulators found!"
    exit 1
fi

echo -e "\nAvailable Booted Simulators:"
i=0
for line in "${lines[@]}"; do
    # Extract UUID (string between braces)
    uuid=$(echo "$line" | sed -E 's/.* \(([0-9A-F-]{36})\).*/\1/')
    # Extract Name (everything before the first parenthesis)
    name=$(echo "$line" | sed -E 's/ \([0-9A-F-]{36}\).*//')
    
    UUIDS[$i]=$uuid
    NAMES[$i]=$name
    
    echo "[$i] $name ($uuid)"
    ((i++))
done
echo "[$i] All Booted Simulators"

echo ""
read -p "Select simulator (0-$i): " SIM_CHOICE

TARGET_UUIDS=()

if [ "$SIM_CHOICE" -eq "$i" ]; then
    TARGET_UUIDS=("${UUIDS[@]}")
elif [ "$SIM_CHOICE" -ge 0 ] && [ "$SIM_CHOICE" -lt "$i" ]; then
    TARGET_UUIDS=("${UUIDS[$SIM_CHOICE]}")
else
    echo "Invalid selection."
    exit 1
fi

echo -e "\n${BLUE}Select Location Preset:${NC}"
echo "1) 42 Paris (48.8966, 2.3185)"
echo "2) Near 42 - 100m away (48.8975, 2.3185)"
echo "3) Near 42 - 500m away (48.9011, 2.3185) [Outside small radius]"
echo "4) New York (40.7128, -74.0060)"
echo "5) Tokyo (35.6762, 139.6503)"
echo "6) Default Location (37.7858, -122.4064)"
echo "7) Custom Coordinates"

read -p "Choice [1]: " LOC_CHOICE
LOC_CHOICE=${LOC_CHOICE:-1}

case $LOC_CHOICE in
    1) LAT=48.8966; LON=2.3185 ;;
    2) LAT=48.8975; LON=2.3185 ;;
    3) LAT=48.9011; LON=2.3185 ;;
    4) LAT=40.7128; LON=-74.0060 ;;
    5) LAT=35.6762; LON=139.6503 ;;
    6) LAT=37.7858; LON=-122.4064 ;;
    7)
        read -p "Enter Latitude: " LAT
        read -p "Enter Longitude: " LON
        ;;
    *)
        echo "Invalid choice. Defaulting to 42 Paris."
        LAT=48.8966; LON=2.3185
        ;;
esac

echo ""
for uuid in "${TARGET_UUIDS[@]}"; do
    echo -e "Setting location for ${GREEN}$uuid${NC} to ${GREEN}$LAT, $LON${NC}..."
    xcrun simctl location "$uuid" set "$LAT,$LON"
done

echo -e "\n${GREEN}Done!${NC} Note: Validating Geo+Time usually requires the app to request location updates again."
