<?php

declare(strict_types=1);

namespace Dimmi\Tests;

use Dimmi\Messenger;
use PHPUnit\Framework\TestCase;

final class MessengerTest extends TestCase
{
    public function testIdentifiesWithAnInlineToken(): void
    {
        $html = Messenger::snippet('wgt_1', 'https://dimmi.test/', 'a.b.c');
        $this->assertSame(
            '<script>window.Dimmi=window.Dimmi||function(){(Dimmi.q=Dimmi.q||[]).push([].slice.call(arguments))};Dimmi("identify",function(){return "a.b.c"});</script>' . "\n"
            . '<script async src="https://dimmi.test/widget/v1/loader.js" data-app-id="wgt_1"></script>' . "\n",
            $html,
        );
    }

    public function testGuestsGetOnlyTheLoader(): void
    {
        $html = Messenger::snippet('wgt_1', 'https://dimmi.test', null);
        $this->assertStringNotContainsString('identify', $html);
        $this->assertStringContainsString('data-app-id="wgt_1"', $html);
    }

    public function testIdentityUrlAndNonceAreEscaped(): void
    {
        $html = Messenger::snippet('wgt_"1', 'https://dimmi.test', '</script><b>', '/dimmi/token?a=1&b=2', 'n"1');
        $this->assertStringContainsString('data-identity-url="/dimmi/token?a=1&amp;b=2"', $html);
        $this->assertStringContainsString('data-app-id="wgt_&quot;1"', $html);
        $this->assertStringContainsString('<script nonce="n&quot;1">', $html);
        $this->assertStringContainsString('\u003C/script\u003E', $html);
        $this->assertSame(2, substr_count($html, '</script>'));
    }
}
