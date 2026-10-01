# Shared image references for the standalone source tree's stack/debug scripts.
aiongo_root=${0:A:h:h:h}
typeset -A aiongo_images
while read -r role release_image source_image; do
    aiongo_images[$role]=$release_image
done < <(python3 "$aiongo_root/scripts/images.py" list)
db_image=${aiongo_images[db]}
login_go_image=${aiongo_images[login-go]}
chat_go_image=${aiongo_images[chat-go]}
game_go_image=${aiongo_images[game-go]}
gamesniff_image=${aiongo_images[gamesniff]}
panel_image=${aiongo_images[panel]}
login_java21_image=${aiongo_images[login-java21]}
chat_java21_image=${aiongo_images[chat-java21]}
game_java21_image=${aiongo_images[game-java21]}
