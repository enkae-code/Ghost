from PIL import Image
import numpy as np

def remove_black_background(input_path, output_path):
    print(f"Processing {input_path}...")
    try:
        img = Image.open(input_path).convert("RGBA")
        data = np.array(img)

        # distinct r, g, b, a variables
        r, g, b, a = data[:,:,0], data[:,:,1], data[:,:,2], data[:,:,3]

        # Calculate brightness
        brightness = (r.astype(float) + g.astype(float) + b.astype(float)) / 3.0
        
        # Assumption: Input is Black Logo on White Background.
        # We want: White Logo on Transparent Background.
        
        # Alpha: The darker the pixel, the more opaque it should be.
        # Brightness 255 (White) -> Alpha 0
        # Brightness 0   (Black) -> Alpha 255
        new_a = (255 - brightness).astype(np.uint8)
        
        # THRESHOLDING:
        # If the original pixel was very bright (background), force alpha to 0.
        # This removes the "grey box" effect from off-white backgrounds.
        new_a[brightness > 230] = 0
        
        # Color: Force all pixels to be White (255, 255, 255)
        # We only rely on Alpha for the shape.
        data[:,:,0] = 255 # R
        data[:,:,1] = 255 # G
        data[:,:,2] = 255 # B
        
        # Update alpha channel
        data[:,:,3] = new_a
        
        # Create new image
        new_img = Image.fromarray(data)
        new_img.save(output_path)
        print(f"Saved transparent image to {output_path}")

    except Exception as e:
        print(f"Error: {e}")

if __name__ == "__main__":
    input_file = r"c:\Developer\Ghost\assets\brand\GhostLogo.png"
    output_file = r"c:\Developer\Ghost\conscience_go\dashboard\landing\public\ghost-logo-transparent.png"
    remove_black_background(input_file, output_file)
