<?php

declare(strict_types=1);

namespace Dimmi;

/**
 * The HTML that boots the dimmi messenger on your pages.
 *
 *     echo Dimmi\Messenger::snippet('wgt_...', 'https://dimmi.acme.com', Dimmi\Identity::sign($user));
 *
 * Pass a token to identify the signed-in user right away, or an identity
 * URL (such as "/dimmi/token") the messenger fetches {"token"} from itself,
 * which keeps long sessions identified. Pass neither for guests.
 */
final class Messenger
{
    public static function snippet(
        string $appId,
        string $host,
        ?string $token = null,
        ?string $identityUrl = null,
        ?string $nonce = null,
    ): string {
        $nonceAttr = $nonce !== null && $nonce !== '' ? ' nonce="' . self::attr($nonce) . '"' : '';
        $src = rtrim($host, '/') . '/widget/v1/loader.js';
        $html = '';

        if ($token !== null && $token !== '') {
            $json = json_encode($token, JSON_HEX_TAG | JSON_HEX_AMP | JSON_HEX_APOS | JSON_HEX_QUOT | JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR);
            $html .= "<script{$nonceAttr}>window.Dimmi=window.Dimmi||function(){(Dimmi.q=Dimmi.q||[]).push([].slice.call(arguments))};"
                . "Dimmi(\"identify\",function(){return {$json}});</script>\n";
        }

        $identity = $identityUrl !== null && $identityUrl !== '' ? ' data-identity-url="' . self::attr($identityUrl) . '"' : '';

        return $html . '<script async src="' . self::attr($src) . '" data-app-id="' . self::attr($appId) . '"' . $identity . $nonceAttr . "></script>\n";
    }

    private static function attr(string $value): string
    {
        return htmlspecialchars($value, ENT_QUOTES | ENT_SUBSTITUTE | ENT_HTML5, 'UTF-8');
    }
}
