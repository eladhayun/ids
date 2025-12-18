<?php
/**
 * The template for displaying the footer.
 *
 * Contains the body & html closing tags.
 *
 * @package HelloElementor
 */

if (!defined('ABSPATH')) {
	exit; // Exit if accessed directly.
}

if (!function_exists('elementor_theme_do_location') || !elementor_theme_do_location('footer')) {
	if (hello_elementor_display_header_footer()) {
		if (did_action('elementor/loaded') && hello_header_footer_experiment_active()) {
			get_template_part('template-parts/dynamic-footer');
		} else {
			get_template_part('template-parts/footer');
		}
	}
}
?>

<?php wp_footer(); ?>

<script>
	/**
	 * IDS Chatbot Widget Integration
	 */
	(function () {
		// Configuration
		const IDS_APP_URL = 'https://ids.jshipster.io';
		const WIDGET_ICON = `
		<img src="https://ids.jshipster.io/static/images/military-chatbot-icon-fav.png" alt="Tactical Support" style="width: 100%; height: 100%; object-fit: cover; border-radius: 50%;">
		`;

		// Styles
		const styles = `
			.ids-widget-fab {
				position: fixed;
				bottom: 100px;
				right: 8px; /* Centered alignment assuming WhatsApp is 60px wide at right:20px */
				width: 85px; 
				height: 85px;
				cursor: pointer;
				z-index: 9999;
				transition: transform 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
				
				/* Force reset of button styles */
				border: none !important;
				background: transparent !important;
				box-shadow: none !important;
				border-radius: 50% !important;
				outline: none !important;
				appearance: none !important;
				-webkit-appearance: none !important;

				padding: 0 !important;
				display: flex;
				align-items: center;
				justify-content: center;
				filter: drop-shadow(0 4px 6px rgba(0,0,0,0.3));
			}
			.ids-widget-tooltip {
				position: absolute;
				right: 90px; /* Left of the button */
				background: rgba(0, 0, 0, 0.8);
				color: white;
				padding: 6px 12px;
				border-radius: 6px;
				font-size: 14px;
				font-family: sans-serif;
				white-space: nowrap;
				opacity: 0;
				pointer-events: none;
				transition: opacity 0.3s;
				font-weight: 500;
				top: 50%;
				transform: translateY(-50%);
			}
			.ids-widget-fab:hover .ids-widget-tooltip {
				opacity: 1;
			}
			.ids-widget-fab:hover {
				transform: scale(1.1);
			}
			.ids-widget-container {
				position: fixed;
				bottom: 170px;
				right: 20px;
				width: 400px;
				height: 600px;
				max-width: calc(100vw - 40px);
				max-height: calc(100vh - 180px);
				border-radius: 12px;
				box-shadow: 0 5px 40px rgba(0,0,0,0.16);
				z-index: 9999;
				overflow: hidden;
				opacity: 0;
				transform: translateY(20px) scale(0.95);
				pointer-events: none;
				transition: all 0.3s ease;
			}
			.ids-widget-container.open {
				opacity: 1;
				transform: translateY(0) scale(1);
				pointer-events: all;
			}
			.ids-widget-iframe {
				width: 100%;
				height: 100%;
				border: none;
				background: white;
			}
		`;

		// Inject Styles
		const styleSheet = document.createElement("style");
		styleSheet.textContent = styles;
		document.head.appendChild(styleSheet);

		// Create FAB
		const fab = document.createElement('button');
		fab.className = 'ids-widget-fab';
		fab.innerHTML = WIDGET_ICON + '<span class="ids-widget-tooltip">AI Chatbot</span>';
		fab.setAttribute('aria-label', 'Open Tactical Support');
		document.body.appendChild(fab);

		// Create Iframe Container
		const container = document.createElement('div');
		container.className = 'ids-widget-container';
		document.body.appendChild(container);

		// State
		let isOpen = false;
		let iframeLoaded = false;

		// Toggle Function
		function toggleChat() {
			isOpen = !isOpen;
			if (isOpen) {
				if (!iframeLoaded) {
					const iframe = document.createElement('iframe');
					iframe.className = 'ids-widget-iframe';
					iframe.src = IDS_APP_URL;
					iframe.setAttribute('allow', 'clipboard-write');
					container.appendChild(iframe);
					iframeLoaded = true;
				}
				container.classList.add('open');
			} else {
				container.classList.remove('open');
			}
		}

		// Event Listeners
		fab.addEventListener('click', toggleChat);
		window.addEventListener('message', function (event) {
			if (event.data && event.data.type === 'ids-close-chat') {
				if (isOpen) toggleChat();
			}
		});
	})();
</script>
</body>

</html>